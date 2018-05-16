package client

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
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
	b, err := json.Marshal(cpuStat)
	if err != nil {
		log.V(2).Infof("%v", err)
		return b, err
	}
	log.V(4).Infof("getCpuUtil, output %v", string(b))
	return b, nil
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

func marshal(data interface{}) ([]byte, error) {
	j, err := json.Marshal(data)
	if err != nil {
		log.V(2).Infof("json marshal error %v", err)
		return nil, err
	}
	log.V(6).Infof("marshal json: %v", string(j))
	return j, nil
}

func GetMemInfo() ([]byte, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		log.V(2).Infof("get memory stat error %v", err)
		return nil, err
	}
	return marshal(vmStat)
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

func init() {
	statsR.buff = make([]*linuxproc.Stat, statsRingCap)
	go pollStats()
}
