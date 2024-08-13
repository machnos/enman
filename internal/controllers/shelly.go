package controllers

import (
	"encoding/json"
	"enman/internal/config"
	"enman/internal/domain"
	"enman/internal/log"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type shellyPvController struct {
	connectUrl     string
	client         *http.Client
	name           string
	model          string
	nrOfSwitches   uint8
	currentEnabled bool
}

func newShellyPvController(config *config.PvController) (domain.PvController, error) {
	url := config.ConnectURL
	if strings.HasSuffix(url, "/") {
		url = url[0 : len(url)-1]
	}

	spc := &shellyPvController{
		connectUrl: url,
		client:     http.DefaultClient,
	}
	return spc, spc.validPvController()
}

func (s *shellyPvController) validPvController() error {
	// First get the number of switches
	result, err := s.executeGet(s.connectUrl + "/rpc/Shelly.GetConfig")
	if err != nil {
		return err
	}
	for i := 0; i < 10; i++ { // start of the execution block
		_, ok := result[fmt.Sprintf("switch:%d", i)]
		if !ok {
			break
		}
		s.nrOfSwitches++
	}

	// Then get the device info
	result, err = s.executeGet(s.connectUrl + "/rpc/Shelly.GetDeviceInfo")
	if err != nil {
		return err
	}
	s.name = result["name"].(string)
	s.model = result["app"].(string)
	if s.nrOfSwitches <= 0 {
		return fmt.Errorf("detected an usupported Shelly %s (%s) PV Controller at %s", s.model, s.name, s.connectUrl)
	}

	// And finally determine the current switch state
	result, err = s.executeGet(s.connectUrl + "/rpc/Switch.GetStatus?id=0")
	if err != nil {
		return err
	}
	s.currentEnabled = result["output"].(bool)

	log.Infof("Detected a Shelly %s (%s) PV Controller at %s", s.model, s.name, s.connectUrl)
	return nil
}

func (s *shellyPvController) executeGet(url string) (map[string]any, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *shellyPvController) DisablePv() error {
	if !s.currentEnabled {
		return nil
	}
	log.Infof("Disabling PV %s", s.name)
	hasError := false
	for i := uint8(0); i < s.nrOfSwitches; i++ {
		_, err := s.executeGet(fmt.Sprintf("%s/rpc/Switch.Set?id=%d&on=false", s.connectUrl, i))
		if err != nil {
			log.Warningf("Unable to disable PV switch %d on %s: %v", i, s.name, err)
			hasError = true
		}
	}
	if !hasError {
		s.currentEnabled = false
	}
	return nil
}

func (s *shellyPvController) EnablePv() error {
	if s.currentEnabled {
		return nil
	}
	log.Infof("Enabling PV %s", s.name)
	hasError := false
	for i := uint8(0); i < s.nrOfSwitches; i++ {
		_, err := s.executeGet(fmt.Sprintf("%s/rpc/Switch.Set?id=%d&on=true", s.connectUrl, i))
		if err != nil {
			log.Warningf("Unable to enable PV switch %d on %s: %v", i, s.name, err)
			hasError = true
		}
	}
	if !hasError {
		s.currentEnabled = true
	}
	return nil
}
