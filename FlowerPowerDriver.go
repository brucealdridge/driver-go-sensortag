package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/gatt"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/model"
)

var activeFlowerPowers = make(map[string]bool)
var announcedFlowerPowers = make(map[string]bool)
var info = ninja.LoadModuleInfo("./package.json")

type FlowerPowerDriver struct {
	config     *FlowerPowerDriverConfig
	conn       *ninja.Connection
	sendEvent  func(event string, payload interface{}) error
	gattClient *gatt.Client
}

type FlowerPowerDriverConfig struct {
	NumberOfDevices int
}

func defaultConfig() *FlowerPowerDriverConfig {
	return &FlowerPowerDriverConfig{
		NumberOfDevices: 0,
	}
}

func NewFlowerPowerDriver() (*FlowerPowerDriver, error) {

	conn, err := ninja.Connect("FlowerPower")

	if err != nil {
		log.Fatalf("Failed to create fake driver: %s", err)
	}

	driver := &FlowerPowerDriver{
		conn: conn,
	}

	err = conn.ExportDriver(driver)

	if err != nil {
		log.Fatalf("Failed to export FlowerPower driver: %s", err)
	}

	return driver, nil
}

func (d *FlowerPowerDriver) foundDevice(device *gatt.DiscoveredDevice) {
	// log.Infof("found device %s", device.Address)
	for uuid, _ := range device.Advertisement.ServiceUuids {
		if uuid == flowerPowerServiceUuid {
			if announcedFlowerPowers[device.Address] {
				return
			}

			log.Infof("Making flower power %s", device.Address)
			fp := NewFlowerPower(d, d.gattClient, device)
			err := d.conn.ExportDevice(fp)
			if err != nil {
				log.Fatalf("Failed to export flowerpower %+v %s", fp, err)
			}

			err = d.conn.ExportChannel(fp, fp.temperatureChannel, "temperature")
			if err != nil {
				log.Fatalf("Failed to export flowerpower temperature channel %s, dumping device info", err)
				spew.Dump(fp)
			}

			err = d.conn.ExportChannel(fp, fp.moistureChannel, "moisture")
			if err != nil {
				log.Fatalf("Failed to export flowerpower moisture channel %s, dumping device info", fp, err)
				spew.Dump(fp)
			}

			err = d.conn.ExportChannel(fp, fp.illuminanceChannel, "illuminance")
			if err != nil {
				log.Fatalf("Failed to export flowerpower illuminance channel %s, dumping device info", err)
				spew.Dump(fp)
			}

			announcedFlowerPowers[device.Address] = true
		}
	}
}

func (d *FlowerPowerDriver) Start(config *FlowerPowerDriverConfig) error {
	log.Infof("Flower Power Driver Starting with config %v", config)

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

	d.gattClient = client
	client.Advertisement = d.foundDevice

	err = client.Start()
	if err != nil {
		log.FatalError(err, "Failed to start client")
	}

	err = client.StartScanning(true)
	if err != nil {
		log.FatalError(err, "Failed to start scanning")
	}

	return nil
}

func (d *FlowerPowerDriver) Stop() error {
	log.Infof("Flower Power Driver stopping")
	return nil
}

func (d *FlowerPowerDriver) GetModuleInfo() *model.Module {
	return info
}

func (d *FlowerPowerDriver) SetEventHandler(sendEvent func(event string, payload interface{}) error) {
	d.sendEvent = sendEvent
}
