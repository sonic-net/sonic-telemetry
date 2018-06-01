package client

import (
	"strings"

	log "github.com/golang/glog"
)

type Bgpinfo struct {
	Version      string
	AsNumber     string
	MsgRcvd      string
	MsgSent      string
	TblVer       string
	InQ          string
	OutQ         string
	UpDown       string
	State_PfxRcd string
}

func GetIprouteNum() ([]byte, error) {
	strout, err := getCommandOut("docker exec -i bgp vtysh -c \"show ip route summary\"")
	if err != nil {
		log.V(2).Infof("show ip route summary error %v", err)
		return nil, err
	}
	stringlist := strings.Split(strout, "\n")
	data := make(map[string]string)
	for i := 0; strings.Split(stringlist[i], " ")[0] != ""; i++ {
		if strings.Fields(stringlist[i])[0] == "Totals" {
			data["Routes"] = strings.Fields(stringlist[i])[1]
			data["FIB"] = strings.Fields(stringlist[i])[2]
		}
	}
	return marshal(data)
}

func GetPrefixNum() ([]byte, error) {
	strout, err := getCommandOut("docker exec -i bgp vtysh -c \"show ip route summary prefix\"")
	if err != nil {
		log.V(2).Infof("show ip route summary prefix error %v", err)
		return nil, err
	}
	stringlist := strings.Split(strout, "\n")
	data := make(map[string]string)
	for i := 0; strings.Split(stringlist[i], " ")[0] != ""; i++ {
		if strings.Fields(stringlist[i])[0] == "Totals" {
			data["PrefixRoutes"] = strings.Fields(stringlist[i])[1]
			data["FIB"] = strings.Fields(stringlist[i])[2]
		}
	}
	return marshal(data)
}

func BgpSummary() ([]byte, error) {
	strout, err := getCommandOut("docker exec -i bgp vtysh -c \"show ip bgp summary\"")
	if err != nil {
		log.V(2).Infof("show ip bgp summary error %v", err)
		return nil, err
	}
	stringlist := strings.Split(strout, "\n")
	data := make(map[string]Bgpinfo)
	findbgpflag := false
	bgpinfoindex := 0
	for i := 0; i < len(stringlist); i++ {
		if strings.Split(stringlist[i], " ")[0] == "Neighbor" {
			bgpinfoindex = i + 1
			findbgpflag = true
			break
		}
	}
	if findbgpflag == true {
		for i := bgpinfoindex; strings.Split(stringlist[i], " ")[0] != ""; i++ {
			data[strings.Fields(stringlist[i])[0]] = Bgpinfo{
				strings.Fields(stringlist[i])[1],
				strings.Fields(stringlist[i])[2],
				strings.Fields(stringlist[i])[3],
				strings.Fields(stringlist[i])[4],
				strings.Fields(stringlist[i])[5],
				strings.Fields(stringlist[i])[6],
				strings.Fields(stringlist[i])[7],
				strings.Fields(stringlist[i])[8],
				strings.Fields(stringlist[i])[9]}
		}
	}
	return marshal(data)
}
