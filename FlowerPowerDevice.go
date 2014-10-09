package main

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"math"
	"strconv"

	"github.com/bitly/go-simplejson"
	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/gatt"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/model"
)

type FlowerPower struct {
	driver             ninja.Driver
	info               *model.Device
	sendEvent          func(event string, payload interface{}) error
	gattClient         *gatt.Client
	gattDevice         *gatt.DiscoveredDevice
	temperatureChannel *channels.TemperatureChannel
	moistureChannel    *channels.MoistureChannel
	illuminanceChannel *channels.IlluminanceChannel
}

func NewFlowerPower(driver ninja.Driver, gattClient *gatt.Client, gattDevice *gatt.DiscoveredDevice) *FlowerPower {

	name := "FlowerPower"

	fp := &FlowerPower{
		driver: driver,
		info: &model.Device{
			NaturalID:     gattDevice.Address,
			NaturalIDType: "FlowerPower",
			Name:          &name, //TODO Fill me in with retrieved value
			Signatures: &map[string]string{
				"ninja:manufacturer": "Parrot",
				"ninja:productName":  "FlowerPower",
				"ninja:productType":  "FlowerPower",
				"ninja:thingType":    "plant sensor",
			},
		},
	}

	fp.temperatureChannel = channels.NewTemperatureChannel(fp)
	fp.moistureChannel = channels.NewMoistureChannel(fp)
	fp.illuminanceChannel = channels.NewIlluminanceChannel(fp)

	fp.gattClient = gattClient
	fp.gattDevice = gattDevice

	gattDevice.Connected = fp.deviceConnected
	gattDevice.Disconnected = fp.deviceDisconnected
	gattDevice.Notification = fp.handleNotification

	err := gattClient.Connect(gattDevice.Address, gattDevice.PublicAddress)
	if err != nil {
		log.Errorf("Connect error:%s", err)
		return nil
	}

	return fp
}

func (fp *FlowerPower) handleNotification(notification *gatt.Notification) {
	if notification.Handle == sunlightHandle {
		sunlight := parseSunlight(notification.Data)
		log.Infof("Got sunlight: %f", sunlight)
		fp.illuminanceChannel.SendState(sunlight)

	} else if notification.Handle == moistureHandle {
		moisture := parseMoisture(notification.Data)
		log.Infof("Got moisture: %f", moisture)
		fp.moistureChannel.SendState(moisture)

	} else if notification.Handle == temperatureHandle {
		temperature := parseTemperature(notification.Data)
		log.Infof("Got temperature: %f", temperature)
		fp.temperatureChannel.SendState(temperature)

	} else {
		log.Infof("Unknown notification handle")
		spew.Dump(notification)
	}
}

func (fp *FlowerPower) notifyAll() {
	fp.notifyByHandle(sunlightStartHandle, sunlightEndHandle)
	fp.notifyByHandle(moistureStartHandle, moistureEndHandle)
	fp.notifyByHandle(temperatureStartHandle, temperatureEndHandle)
}

func (fp *FlowerPower) deviceConnected() {
	activeFlowerPowers[fp.gattDevice.Address] = true
	log.Infof("Connected to flower power: %s", fp.gattDevice.Address)
	log.Infof("Setting up notifications")
	fp.notifyAll()
	log.Infof("Enabling live mode")
	fp.EnableLiveMode()

}

func (fp *FlowerPower) deviceDisconnected() {
	log.Infof("Disconnected from flower power: %s", fp.gattDevice.Address)
	activeFlowerPowers[fp.gattDevice.Address] = false
	err := fp.gattClient.Connect(fp.gattDevice.Address, fp.gattDevice.PublicAddress)
	if err != nil {
		log.Errorf("reconnect error:%s", err)
	}
}

func (fp *FlowerPower) GetDeviceInfo() *model.Device {
	return fp.info
}

func (fp *FlowerPower) GetDriver() ninja.Driver {
	return fp.driver
}

func (fp *FlowerPower) SetEventHandler(sendEvent func(event string, payload interface{}) error) {
	fp.sendEvent = sendEvent
}

func (fp *FlowerPower) EnableLiveMode() {
	//this sends raw bytes necessary to put device into live mode
	fp.gattClient.SetupFlowerPower(fp.gattDevice.Address) //TODO FIX ASAP!
}

func (fp *FlowerPower) notifyByHandle(startHandle, endHandle uint16) {
	fp.gattClient.Notify(fp.gattDevice.Address, true, startHandle, endHandle, true, false)
}

func parseSunlight(data []byte) float64 {
	sensorVal := bytesToUint(data)
	if sensorVal < 0 {
		sensorVal = 0
	} else if sensorVal > 65530 {
		sensorVal = 65530
	}
	rounded := math.Floor(float64(sensorVal)/10) * 10 //Only 10% of data in mapping

	return getValFromMap("sunlight.json", rounded)
}

func parseMoisture(data []byte) float64 {
	sensorVal := bytesToUint(data)
	if sensorVal < 210 {
		sensorVal = 210
	} else if sensorVal > 700 {
		sensorVal = 700
	}
	return getValFromMap("soil-moisture.json", float64(sensorVal))
}

func parseTemperature(data []byte) float64 {
	sensorVal := bytesToUint(data)
	if sensorVal < 210 {
		sensorVal = 210
	} else if sensorVal > 1372 {
		sensorVal = 1372
	}
	return getValFromMap("temperature.json", float64(sensorVal))
}

func (fp *FlowerPower) getValFromHandle(handle int) []byte {
	// log.Debugf("--Readbyhandle-- address: %s handle: %d", fp.gattDevice.Address, handle)
	data := <-fp.gattClient.ReadByHandle(fp.gattDevice.Address, uint16(handle))
	return data
}

func getValFromMap(filename string, sensorVal float64) float64 {
	mapFile, err := ioutil.ReadFile("data/" + filename)
	if err != nil {
		log.Fatalf("Error reading %s json map file: %s", filename, err)
	}

	mapJson, err := simplejson.NewFromReader(bytes.NewBuffer(mapFile))
	if err != nil {
		log.Fatalf("Error creating reader: %s", err)
	}

	sensorValStr := strconv.Itoa(int(sensorVal))

	ret, err := mapJson.Get(sensorValStr).Float64()

	if err != nil {
		log.Infof("Error parsing sensor value %f with stringified value %s to mapped value: %s", sensorVal, sensorValStr, err)
	}

	return ret
}

func bytesToUint(in []byte) uint16 {
	var ret uint16
	buf := bytes.NewReader(in)
	err := binary.Read(buf, binary.LittleEndian, &ret)
	if err != nil {
		log.Errorf("bytesToUint: Couldn't convert bytes % X to uint", in)
		return 0
	}
	return ret
}

func (fp *FlowerPower) GetSunlight() float64 {
	sensorVal := fp.getValFromHandle(sunlightHandle)
	return parseSunlight(sensorVal)
}

func (fp *FlowerPower) GetTemperature() float64 {
	sensorVal := fp.getValFromHandle(temperatureHandle)
	return parseTemperature(sensorVal)
}

func (fp *FlowerPower) GetMoisture() float64 {
	sensorVal := fp.getValFromHandle(moistureHandle)
	return parseMoisture(sensorVal)
}

func (fp *FlowerPower) GetBatteryLevel() float64 {
	value := bytesToUint(fp.getValFromHandle(batteryHandle))
	return float64(value)
}
