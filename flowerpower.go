package main

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"math"
	"strconv"

	"github.com/bitly/go-simplejson"
	"github.com/ninjasphere/gatt"
	// "github.com/davecgh/go-spew/spew"
)

func getValFromHandle(client *gatt.Client, device *gatt.DiscoveredDevice, handle int) uint16 {
	data := <-client.ReadByHandle(device.Address, uint16(handle))
	var ret uint16
	buf := bytes.NewReader(data[1:])
	err := binary.Read(buf, binary.LittleEndian, &ret)
	if err != nil {
		log.Infof("Got error reading flower power: %s", err)
	}
	return ret
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

func GetSunlight(client *gatt.Client, device *gatt.DiscoveredDevice) float64 {
	sensorVal := getValFromHandle(client, device, 37)
  if (sensorVal < 0) {
    sensorVal = 0;
  } else if (sensorVal > 65530) {
    sensorVal = 65530;
  }
	rounded := math.Floor(float64(sensorVal)/10) * 10 //Only 10% of data in mapping
	return getValFromMap("sunlight.json", rounded)
}

func GetTemperature(client *gatt.Client, device *gatt.DiscoveredDevice) float64 {
	sensorVal := getValFromHandle(client, device, 49)
  if (sensorVal < 210) {
    sensorVal = 210;
  } else if (sensorVal > 1372) {
    sensorVal = 1372;
  }
	return getValFromMap("temperature.json", float64(sensorVal))
}

func GetMoisture(client *gatt.Client, device *gatt.DiscoveredDevice) float64 {
  sensorVal := getValFromHandle(client, device, 53)

  if (sensorVal < 210) {
    sensorVal = 210
  } else if (sensorVal > 700) {
    sensorVal = 700
  }

  return getValFromMap("soil-moisture.json", float64(sensorVal))
}

func GetBatteryLevel(client *gatt.Client, device *gatt.DiscoveredDevice) float64 {
    return float64(getValFromHandle(client, device, 68))
}
