package server

import (
	"bitbucket.org/raylios/cloudpost-go/slog"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func (ser *server) portMapHandler(w http.ResponseWriter, r *http.Request) {

	slog.Debug(">>> %v %v, %v", r.Method, r.URL.Path, r.RemoteAddr)

	// body
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {

		fmt.Println("Error while parsing body in portMapHandler: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		slog.Err("ioutil.ReadAll: %v", err)
		return

	}
	slog.Debug(">>> %v", string(body))

	// Parse parameters
	var dat map[string]interface{}
	if err := json.Unmarshal(body, &dat); err != nil {

		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		slog.Err("Error Unmarshaling request body", err)
		return

	}

	// app and camera trunks
	appTracks := dat["app"].(map[string]interface{})
	cameraTracks := dat["camera"].(map[string]interface{})

	// Parse IP Addesses
	camIPs := cameraTracks["addresses"].([]interface{})
	appIPs := appTracks["addresses"].([]interface{})
	camIP := camIPs[0].(string)
	appIP := appIPs[0].(string)

	// Reshape Ports from Tracks
	appPorts := make(map[string]interface{})
	cameraPorts := make(map[string]interface{})

	// Parse App Tracks
	tracks := appTracks["tracks"].([]interface{})
	for _, ifaceTrack := range tracks {
		track := ifaceTrack.(map[string]interface{})
		trackName := track["name"].(string)
		appPorts[trackName] = track["ports"].([]interface{})
	}

	// Parse Camera Tracks
	tracks = cameraTracks["tracks"].([]interface{})
	for _, ifaceTrack := range tracks {
		track := ifaceTrack.(map[string]interface{})
		trackName := track["name"].(string)
		cameraPorts[trackName] = track["ports"].(interface{})
	}

	addTrack(camIP, appIP, cameraPorts["video"].([]interface{}), appPorts["video"].([]interface{}), "video")
	addTrack(camIP, appIP, cameraPorts["audio"].([]interface{}), appPorts["audio"].([]interface{}), "audio")
	addTrack(appIP, camIP, appPorts["talk"].([]interface{}), cameraPorts["talk"].([]interface{}), "talk")

}

func addTrack(mediaSenderAddr, mediaRecvAddr string, mediaSenderPorts, mediaRecvPorts []interface{}, trackType string) {

	slog.Debug("sender addr: %v; receiver addr: %v", mediaSenderAddr, mediaRecvAddr)
	slog.Debug("sender ports: %v, %v", mediaSenderPorts[0], mediaSenderPorts[1])
	slog.Debug("receiver ports: %v, %v", mediaRecvPorts[0], mediaRecvPorts[1])

	mediaSenderAddrPort := mediaSenderAddr + ":" + strconv.Itoa(int(mediaSenderPorts[0].(float64)))
	mediaRecvAddrPort := mediaRecvAddr + ":" + strconv.Itoa(int(mediaRecvPorts[1].(float64)))

	if trackType != "talk" {
		addPortMap(mediaSenderAddrPort, mediaRecvAddr+":"+strconv.Itoa(int(mediaRecvPorts[0].(float64))), RTP, "")
		addPortMap(mediaRecvAddrPort, mediaSenderAddr+":"+strconv.Itoa(int(mediaSenderPorts[1].(float64))), RTCP, mediaSenderAddrPort)
	} else {
		// work around to avoid talk stream been recycled
		addPortMap(mediaSenderAddrPort, mediaRecvAddr+":"+strconv.Itoa(int(mediaRecvPorts[0].(float64))), RTCP, mediaRecvAddrPort)
		addPortMap(mediaRecvAddrPort, mediaSenderAddr+":"+strconv.Itoa(int(mediaSenderPorts[1].(float64))), RTP, "")
	}

}

func addPortMap(mediaSenderAddr, mediaRecvAddr string, portType StreamType, pairAddr string) {

	// check if duplicate
	var udpRelay UdpRelay
	if udpRelay, ok := UdpRelayMap[mediaSenderAddr]; ok {

		recvList := udpRelay.addrList
		for _, addr := range recvList {
			strAddr := addr.IP.String() + ":" + strconv.Itoa(addr.Port)
			if mediaRecvAddr == strAddr {
				slog.Debug("dest port exist, skip addPortMap")
				return
			}
		}

	}

	addrPair := strings.Split(mediaRecvAddr, ":")
	intPort, err := strconv.Atoi(addrPair[1])
	if err != nil || intPort <= 0 {
		slog.Debug("Error parse port in addPortMap: %v", err)
		return
	}

	destAddr := net.UDPAddr{
		IP:   net.ParseIP(addrPair[0]),
		Port: intPort,
	}

	// addPortMap
	udpRelay.addrList = append(udpRelay.addrList, destAddr)
	udpRelay.streamType = portType
	UdpRelayMap[mediaSenderAddr] = udpRelay
	slog.Debug("destAddr: %v", destAddr)
	slog.Debug("UdpRelayMap: %v", UdpRelayMap)

	if portType == RTCP {

		var rtcpStatus = RtcpStatus{
			active:   true,
			pairAddr: pairAddr,
		}
		RtcpStatusMap[mediaSenderAddr] = rtcpStatus

	}
	slog.Debug("RtcpStatusMap: %v", RtcpStatusMap)

}
