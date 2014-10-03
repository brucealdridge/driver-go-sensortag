package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	// "time"
	"github.com/bitly/go-simplejson"
	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/gatt"
	"github.com/ninjasphere/go-ninja"
	"github.com/ninjasphere/go-ninja/logger"
)

const driverName = "driver-flowerpower"

const flowerPowerServiceUuid = "39e1fa0084a811e2afba0002a5d5c51b"

const LIVE_MODE_UUID = "39e1fa0684a811e2afba0002a5d5c51b"
const SUNLIGHT_UUID = "39e1fa0184a811e2afba0002a5d5c51b"
const TEMPERATURE_UUID = "39e1fa0484a811e2afba0002a5d5c51b"
const SOIL_MOISTURE_UUID = "39e1fa0584a811e2afba0002a5d5c51b"
const FRIENDLY_NAME_UUID = "39e1fe0384a811e2afba0002a5d5c51b"
const COLOR_UUID = "39e1fe0484a811e2afba0002a5d5c51b"

type waypointPayload struct {
	Sequence    uint8
	AddressType uint8
	Rssi        int8
	Valid       uint8
}

type adPacket struct {
	Device   string `json:"device"`
	Waypoint string `json:"waypoint"`
	Rssi     int8   `json:"rssi"`
	IsSphere bool   `json:"isSphere"`
}

// configure the agent logger
var log = logger.GetLogger("driver-go-flowerpower")

//var mesh *udpMesh

func sendRssi(device string, name string, waypoint string, rssi int8, isSphere bool, conn *ninja.NinjaConnection) {
	device = strings.ToUpper(device)

	// log.Debugf(">> Device:%s Waypoint:%s Rssi: %d", device, waypoint, rssi)

	packet, _ := simplejson.NewJson([]byte(`{
		"params": [
				{
						"device": "",
						"waypoint": "",
						"rssi": 0,
						"isSphere": true
				}
		],
		"time": 0,
		"jsonrpc": "2.0"
}`))

	packet.Get("params").GetIndex(0).Set("device", device)
	if name != "" {
		packet.Get("params").GetIndex(0).Set("name", name)
	}
	packet.Get("params").GetIndex(0).Set("waypoint", waypoint)
	packet.Get("params").GetIndex(0).Set("rssi", rssi)
	packet.Get("params").GetIndex(0).Set("isSphere", isSphere)

	//spew.Dump(packet)
	conn.PublishMessage("$device/"+device+"/TEMPPATH/rssi", packet)

}

func main() {
	os.Exit(realMain())
}

func realMain() int {

	log.Infof("Starting " + driverName)

	conn, err := ninja.Connect("com.ninjablocks.flowerpower")

	if err != nil {
		log.FatalErrorf(err, "Could not connect to MQTT Broker")
	}

	/*if mesh, err = newUdpMesh("239.255.12.34:12345", func(packet *adPacket) {

		spew.Dump("Got mesh packet", packet)

	}); err != nil {
		log.FatalErrorf(err, "Could not connect to UDP mesh")
	}*/

	statusJob, err := ninja.CreateStatusJob(conn, driverName)

	if err != nil {
		log.FatalErrorf(err, "Could not setup status job")
	}

	statusJob.Start()

	out, err := exec.Command("hciconfig").Output()
	if err != nil {
		log.Errorf(fmt.Sprintf("Error: %s", err))
	}
	re := regexp.MustCompile("([0-9A-F]{2}\\:{0,1}){6}")
	mac := strings.Replace(re.FindString(string(out)), ":", "", -1)
	log.Infof("The local mac is %s\n", mac)

	client := &gatt.Client{
		StateChange: func(newState string) {
			log.Infof("Client state change: %s", newState)
		},
	}

	// Waypoint notification characteristic {
	// 	"startHandle": 45,
	// 	"properties": 16, (useNotify = true, useIndicate = false)
	// 	"valueHandle": 46,
	// 	"uuid": "fff4",
	// 	"endHandle": 48,
	// }

	//
	// go func() {
	// 	for {
	// 		time.Sleep(time.Second)
	// 		waypoints := 0
	// 		for id, active := range activeWaypoints {
	// 			// log.Debugf("Waypoint %s is active? %t", id, active)
	// 			if active {
	// 				waypoints++
	// 			}
	// 		}
	// 		// log.Debugf("%d waypoint(s) active", waypoints)
	//
	// 		packet, _ := simplejson.NewJson([]byte(fmt.Sprintf("%d", waypoints)))
	//
	// 		conn.PublishMessage("$location/waypoints", packet)
	// 	}
	// }()

	client.Rssi = func(address string, name string, rssi int8) {
		//log.Printf("Rssi update address:%s rssi:%d", address, rssi)
		sendRssi(strings.Replace(address, ":", "", -1), name, mac, rssi, true, conn)
		//spew.Dump(device);
	}

	client.Advertisement = func(device *gatt.DiscoveredDevice) {
		// log.Debugf("Discovered address:%s rssi:%d", device.Address, device.Rssi)

		// if device.Advertisement.LocalName == "NinjaSphereWaypoint" {
		// 	handleSphereWaypoint(conn, client, device)
		// 	return
		// }

		for uuid, _ := range device.Advertisement.ServiceUuids {
			if uuid == flowerPowerServiceUuid {
				// log.Infof("using uuid %s ", flowerPowerServiceUuid)
				handleFlowerPower(conn, client, device)
			}
		}

	}

	err = client.Start()

	if err != nil {
		log.FatalError(err, "Failed to start client")
	}

	err = client.StartScanning(true)
	if err != nil {
		log.FatalError(err, "Failed to start scanning")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	log.Infof("Got signal:", s)

	return 0
}

var activeWaypoints map[string]bool

func handleSphereWaypoint(conn *ninja.NinjaConnection, client *gatt.Client, device *gatt.DiscoveredDevice) {
	if activeWaypoints[device.Address] {
		return
	}

	if device.Connected == nil {
		device.Connected = func() {
			// log.Infof("Connected to waypoint: %s", device.Address)
			//spew.Dump(device.Advertisement)

			// XXX: Yes, magic numbers.... this enables the notification from our Waypoints
			client.Notify(device.Address, true, 45, 48, true, false)
		}

		device.Disconnected = func() {
			log.Infof("Disconnected from waypoint: %s", device.Address)

			activeWaypoints[device.Address] = false
		}

		device.Notification = func(notification *gatt.Notification) {
			//log.Printf("Got the notification!")

			//XXX: Add the ieee into the payload somehow??
			var payload waypointPayload
			err := binary.Read(bytes.NewReader(notification.Data), binary.LittleEndian, &payload)
			if err != nil {
				log.Errorf("Failed to read waypoint payload : %s", err)
			}

			//	ieee := net.HardwareAddr(reverse(notification.Data[4:]))

			//spew.Dump("ieee:", payload)

			packet := &adPacket{
				Device:   fmt.Sprintf("%x", reverse(notification.Data[4:])),
				Waypoint: strings.Replace(device.Address, ":", "", -1),
				Rssi:     payload.Rssi,
				IsSphere: false,
			}

			sendRssi(packet.Device, "", packet.Waypoint, packet.Rssi, packet.IsSphere, conn)
			//mesh.send(packet)
		}
	}

	err := client.Connect(device.Address, device.PublicAddress)
	if err != nil {
		log.Errorf("Connect error:%s", err)
		return
	}

	activeWaypoints[device.Address] = true
}

var activeFlowerPowers = make(map[string]bool)

func handleFlowerPower(conn *ninja.NinjaConnection, client *gatt.Client, device *gatt.DiscoveredDevice) {

	if activeFlowerPowers[device.Address] {
		return
	}

	if device.Connected == nil {
		device.Connected = func() {
			log.Infof("Connected to flower power: %s", device.Address)
			client.SetupFlowerPower(device.Address)
			log.Infof("Sunlight: %f", GetSunlight(client, device))
			log.Infof("Temperature: %f ", GetTemperature(client, device))
			log.Infof("Moisture: %f", GetMoisture(client, device))



			// XXX: Yes, magic numbers.... this enables the notification from our Waypoints
			// client.Notify(device.Address, true, 45, 48, true, false)

			// Getting Temperature
			// time.Sleep(2*time.Second)
			//
			// data := <-client.ReadByHandle(device.Address, uint16(37))
			// log.Infof("raw data--- % X", data)
			// var sunlight uint16
			// buf := bytes.NewReader(data[1:])
			// err := binary.Read(buf, binary.LittleEndian, &sunlight)
			// if err != nil {
			// 	log.Infof("cant read uint data")
			// }
			// log.Infof("uint data %d", sunlight)

		}

		device.Disconnected = func() {
			log.Infof("Disconnected from flower power: %s", device.Address)

			activeFlowerPowers[device.Address] = false
		}

		device.Notification = func(notification *gatt.Notification) {
			log.Infof("Got the notification!")
			spew.Dump(notification)
		}
	}

	err := client.Connect(device.Address, device.PublicAddress)
	if err != nil {
		log.Errorf("Connect error:%s", err)
		return
	}

	activeFlowerPowers[device.Address] = true

	log.Infof("Found NEW Flower Power %s", device.Address)

	return
}

// reverse returns a reversed copy of u.
func reverse(u []byte) []byte {
	l := len(u)
	b := make([]byte, l)
	for i := 0; i < l/2+1; i++ {
		b[i], b[l-i-1] = u[l-i-1], u[i]
	}
	return b
}
