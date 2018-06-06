package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"

	linuxproc "github.com/c9s/goprocinfo/linux"
	log "github.com/golang/glog"
)

type statsRing struct {
	writeIdx uint64 // slot index to write next
	buff     []*linuxproc.Stat
	mu       sync.RWMutex // Mutex for data protection
}

type cpuStat struct {
	CpuUsageAll cpuUtil   `json:"cpu_all"`
	CpuUsage    []cpuUtil `json:"cpus"`
}

// Cpu utilization rate
type cpuUtil struct {
	Id            string `json:"id"`
	CpuUtil_100ms uint64 `json:"100ms"`
	CpuUtil_1s    uint64 `json:"1s"`
	CpuUtil_5s    uint64 `json:"5s"`
	CpuUtil_1min  uint64 `json:"1min"`
	CpuUtil_5min  uint64 `json:"5min"`
}

type RXTX struct {
	InPut  string
	OutPut string
}

var rxtxdata = make(map[string]RXTX)

const statsRingCap uint64 = 3000 // capacity of statsRing.
var statsR statsRing

func getCpuUtilPercents(cur, last *linuxproc.CPUStat) uint64 {
	curTotal := (cur.User + cur.Nice + cur.System + cur.Idle + cur.IOWait + cur.IRQ + cur.SoftIRQ + cur.Steal + cur.Guest + cur.GuestNice)
	lastTotal := (last.User + last.Nice + last.System + last.Idle + last.IOWait + last.IRQ + last.SoftIRQ + last.Steal + last.Guest + last.GuestNice)
	idleTicks := cur.Idle - last.Idle
	totalTicks := curTotal - lastTotal
	return 100 * (totalTicks - idleTicks) / totalTicks
}

func getCpuUtilStat() *cpuStat {

	stat := cpuStat{}
	statsR.mu.RLock()
	defer statsR.mu.RUnlock()

	current := (statsR.writeIdx + statsRingCap - 1) % statsRingCap
	// Get cpu utilization rate within last 100ms
	last := (statsR.writeIdx + statsRingCap - 2) % statsRingCap
	if statsR.buff[last] == nil {
		return &stat
	}

	curCpuStat := statsR.buff[current].CPUStatAll
	lastCpuStat := statsR.buff[last].CPUStatAll

	CpuUtil_100ms := getCpuUtilPercents(&curCpuStat, &lastCpuStat)
	stat.CpuUsageAll.Id = curCpuStat.Id
	stat.CpuUsageAll.CpuUtil_100ms = CpuUtil_100ms
	for i, cStat := range statsR.buff[last].CPUStats {
		CpuUtil_100ms = getCpuUtilPercents(&statsR.buff[current].CPUStats[i], &cStat)
		stat.CpuUsage = append(stat.CpuUsage, cpuUtil{Id: cStat.Id, CpuUtil_100ms: CpuUtil_100ms})
	}

	// Get cpu utilization rate within last 1s (10*100ms)
	last = (statsR.writeIdx + statsRingCap - 10) % statsRingCap
	if statsR.buff[last] == nil {
		return &stat
	}
	lastCpuStat = statsR.buff[last].CPUStatAll
	CpuUtil_1s := getCpuUtilPercents(&curCpuStat, &lastCpuStat)
	stat.CpuUsageAll.CpuUtil_1s = CpuUtil_1s
	for i, cStat := range statsR.buff[last].CPUStats {
		CpuUtil_1s = getCpuUtilPercents(&statsR.buff[current].CPUStats[i], &cStat)
		stat.CpuUsage[i].CpuUtil_1s = CpuUtil_1s
	}

	// Get cpu utilization rate within last 5s (50*100ms)
	last = (statsR.writeIdx + statsRingCap - 50) % statsRingCap
	if statsR.buff[last] == nil {
		return &stat
	}
	lastCpuStat = statsR.buff[last].CPUStatAll
	CpuUtil_5s := getCpuUtilPercents(&curCpuStat, &lastCpuStat)
	stat.CpuUsageAll.CpuUtil_5s = CpuUtil_5s
	for i, cStat := range statsR.buff[last].CPUStats {
		CpuUtil_5s = getCpuUtilPercents(&statsR.buff[current].CPUStats[i], &cStat)
		stat.CpuUsage[i].CpuUtil_5s = CpuUtil_5s
	}

	// Get cpu utilization rate within last 1m (600*100ms)
	last = (statsR.writeIdx + statsRingCap - 600) % statsRingCap
	if statsR.buff[last] == nil {
		return &stat
	}
	lastCpuStat = statsR.buff[last].CPUStatAll
	CpuUtil_1min := getCpuUtilPercents(&curCpuStat, &lastCpuStat)
	stat.CpuUsageAll.CpuUtil_1min = CpuUtil_1min
	for i, cStat := range statsR.buff[last].CPUStats {
		CpuUtil_1min = getCpuUtilPercents(&statsR.buff[current].CPUStats[i], &cStat)
		stat.CpuUsage[i].CpuUtil_1min = CpuUtil_1min
	}

	// Get cpu utilization rate within last 5m (5*600*100ms)
	last = (statsR.writeIdx + statsRingCap - 30000) % statsRingCap
	if statsR.buff[last] == nil {
		return &stat
	}
	lastCpuStat = statsR.buff[last].CPUStatAll
	CpuUtil_5min := getCpuUtilPercents(&curCpuStat, &lastCpuStat)
	stat.CpuUsageAll.CpuUtil_5min = CpuUtil_5min
	for i, cStat := range statsR.buff[last].CPUStats {
		CpuUtil_5min = getCpuUtilPercents(&statsR.buff[current].CPUStats[i], &cStat)
		stat.CpuUsage[i].CpuUtil_5min = CpuUtil_5min
	}
	return &stat
}

func GetCpuUtil() ([]byte, error) {
	cpuStat := getCpuUtilStat()
	log.V(4).Infof("getCpuUtil, cpuStat %v", cpuStat)
	return marshal(cpuStat)
}

func pollStats() {
	for {
		stat, err := linuxproc.ReadStat("/proc/stat")
		if err != nil {
			log.V(2).Infof("stat read fail")
			continue
		}

		statsR.mu.Lock()

		statsR.buff[statsR.writeIdx] = stat
		statsR.writeIdx++
		statsR.writeIdx %= statsRingCap
		statsR.mu.Unlock()
		time.Sleep(time.Millisecond * 100)
	}
}

func getcountersdb(key string, filed string) (string, error) {
	redisDb := Target2RedisDb["COUNTERS_DB"]
	val, err := redisDb.HGet(key, filed).Result()
	if err != nil {
		log.V(2).Infof("redis HGet failed for %v %v", key, filed)
		return val, err
	}
	return val, err
}
func getrate(oldstr string, newstr string, delta int) (rate string) {
	old, _ := strconv.Atoi(oldstr)
	new, _ := strconv.Atoi(newstr)
	rateint := float32((new - old) / delta)
	if rateint > 1024*1024*10 {
		rate = fmt.Sprintf("%.2f MB/s", rateint/1024/1024)
	} else if rateint > 1024*10 {
		rate = fmt.Sprintf("%.2f KB/s", rateint/1024)
	} else {
		rate = fmt.Sprintf("%.2f B/s", rateint)
	}
	return rate
}

func GetRxTxRateTimer() {
	interval := 10
	oldcounters := make(map[string]RXTX)
	newcounters := make(map[string]RXTX)
	countersNameMap := make(map[string]string)
	for {
		time.Sleep(time.Duration(interval) * time.Second)
		countersNameMaptemp, err := getCountersMap("COUNTERS_PORT_NAME_MAP")
		if (err != nil) || (len(countersNameMaptemp) == 0) {
			continue
		}
		countersNameMap = countersNameMaptemp
		break
	}
	for {
		for port, oid := range countersNameMap {
			inputoctet, _ := getcountersdb("COUNTERS:"+oid, "SAI_PORT_STAT_IF_IN_OCTETS")
			outputoctet, _ := getcountersdb("COUNTERS:"+oid, "SAI_PORT_STAT_IF_OUT_OCTETS")
			oldcounters[port] = RXTX{inputoctet, outputoctet}
		}
		time.Sleep(time.Duration(interval) * time.Second)
		for port, oid := range countersNameMap {
			inputoctet, _ := getcountersdb("COUNTERS:"+oid, "SAI_PORT_STAT_IF_IN_OCTETS")
			outputoctet, _ := getcountersdb("COUNTERS:"+oid, "SAI_PORT_STAT_IF_OUT_OCTETS")
			newcounters[port] = RXTX{inputoctet, outputoctet}
			inputrate := getrate(oldcounters[port].InPut, newcounters[port].InPut, interval)
			outputrate := getrate(oldcounters[port].OutPut, newcounters[port].OutPut, interval)
			rxtxdata[port] = RXTX{inputrate, outputrate}
		}
	}
}

func GetRxTxRate() ([]byte, error) {
	return marshal(rxtxdata)
}

func GetConfigdb() ([]byte, error) {
	redisDb := Target2RedisDb["CONFIG_DB"]
	dbkeys, err := redisDb.Keys("*").Result()
	data1 := make(map[string]map[string]map[string]interface{})
	if err != nil {
		log.V(2).Infof("redis Keys failed with err %v", err)
		return nil, err
	}
	separator, _ := GetTableKeySeparator("CONFIG_DB")
	for _, dbkey := range dbkeys {
		data2 := make(map[string]map[string]interface{})
		log.V(6).Infof("\r\n dbkey= %v \r\n", dbkey)
		dbkeystr := strings.Split(dbkey, separator)
		if len(dbkeystr) <= 1 {
			log.V(2).Infof("not get key %v value", dbkeystr)
			continue
		}
		table := dbkeystr[0]
		fild := strings.Join(dbkeystr[1:], separator)
		fv, err := redisDb.HGetAll(dbkey).Result()
		if err != nil {
			log.V(2).Infof("redis HGetAll failed dbkey %s", dbkey)
			return nil, err
		}
		data3 := make(map[string]interface{})
		for f, v := range fv {
			if strings.HasSuffix(f, "@") {
				f = strings.Replace(f, "@", "", -1)
				vlist := strings.Split(v, ",")
				data3[f] = vlist
			} else {
				data3[f] = v
			}
		}
		data2[fild] = data3
		if _, exist := data1[table]; exist {
			data1[table][fild] = data3
		} else {
			data1[table] = data2
		}
	}
	log.V(6).Infof("\r\n data1= %v \r\n", data1)
	return marshal(data1)
}

func marshal(data interface{}) ([]byte, error) {
	j, err := json.Marshal(data)
	if err != nil {
		log.V(2).Infof("json marshal error %v", err)
		return nil, err
	}
	log.V(6).Infof("marshal json:\n %v", string(j))
	return j, nil
}

func GetMemInfo() ([]byte, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		log.V(2).Infof("get memory stat error %v", err)
		return nil, err
	}
	data := map[string]string{
		"UsedPercent": strconv.FormatFloat(vmStat.UsedPercent, 'f', 2, 64),
	}
	return marshal(data)
}

func GetDiskUsage() ([]byte, error) {
	diskStat, err := disk.Usage("/")
	if err != nil {
		log.V(2).Infof("get disk stat error %v", err)
		return nil, err
	}
	data := map[string]string{
		"UsedPercent": strconv.FormatFloat(diskStat.UsedPercent, 'f', 2, 64),
	}
	return marshal(data)
}

// Get version and build date from sonic_version.yml
func GetVersion() ([]byte, error) {
	ver, err := ioutil.ReadFile("/etc/sonic/sonic_version.yml")
	if err != nil {
		log.V(2).Infof("read version file error %v", err)
		return nil, err
	}

	m := make(map[string]string)
	err = yaml.Unmarshal([]byte(ver), &m)
	if err != nil {
		log.V(2).Infof("unmarshal version yaml error %v", err)
		return nil, err
	}

	data := map[string]string{}
	if val, ok := m["build_version"]; ok {
		data["Software Version"] = val
	}
	if val, ok := m["build_date"]; ok {
		data["Build Date"] = val
	}
	return marshal(data)
}

// Get linux command output
func getCommandOut(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	outStr, errStr := string(stdout.Bytes()), string(stderr.Bytes())
	if errStr != "" {
		log.V(2).Infof("exec command %v err: %v", cmd, errStr)
		return "", err
	}
	return outStr, nil
}

// Get ntp stat by linux command.
// Command output have two type:
// (1)
// unsynchronised
//   time server re-starting
//   polling server every 8 s
// (2)
// synchronised to NTP server (10.65.254.222) at stratum 4
//   time correct to within 880 ms
//    polling server every 64 s
// Return json format:
// {
//     "stat":"unsynchronised"
// }
func GetNtpStat() ([]byte, error) {
	// If ntpstat is unsynchronised, command exist status is not 0.
	outStr, err := getCommandOut("ntpstat || true")
	if err != nil {
		return nil, err
	}

	data := map[string]string{}
	if strings.HasPrefix(outStr, "unsynchronised") {
		data["stat"] = "unsynchronised"
	} else if strings.HasPrefix(outStr, "synchronised") {
		data["stat"] = "synchronised"
	} else {
		return nil, fmt.Errorf("invalid result: %v", outStr)
	}
	return marshal(data)
}

// Get last down reason.
// Get the two most recent shutdowns or reboots by "last" command.
// Reboot denotes the system booting up; whereas, shutdown denotes the system going down.
// So a graceful shutdown would show up as reboot preceded by shutdown.
// In contrast, an ungraceful shutdown can be inferred by the omission of shutdown.
func GetDownReason() ([]byte, error) {
	outStr, err := getCommandOut("last -n2 -x shutdown reboot")
	if err != nil {
		return nil, err
	}

	log.V(6).Infof("out: %v", outStr)
	lines := strings.Split(outStr, "\n")
	rebootCnt := 0
	shutdownCnt := 0
	shutdownLine := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "reboot ") {
			rebootCnt += 1
		} else if strings.HasPrefix(line, "shutdown ") {
			shutdownCnt += 1
			shutdownLine = strings.TrimPrefix(line, "shutdown ")
		}
	}

	last := "Unknown"
	date := ""
	if shutdownCnt == 1 && rebootCnt == 1 {
		d := strings.Split(shutdownLine, "  ")
		last = d[0]
		date = d[2]
	}

	data := map[string]string{
		"last": last,
		"date": date,
	}
	return marshal(data)
}

func init() {
	statsR.buff = make([]*linuxproc.Stat, statsRingCap)
	go pollStats()
	go GetRxTxRateTimer()
}
