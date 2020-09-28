package charger

import (
	"fmt"
	"net/http"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
)

const (
	apiSettings     apiFunction = "settings"
	apiMeasurements apiFunction = "measurements"
)

// NRGResponse is the API response if status not OK
type NRGResponse struct {
	Message string
}

// NRGMeasurements is the /api/measurements response
type NRGMeasurements struct {
	ChargingEnergy        float64
	ChargingEnergyOverAll float64
	ChargingPower         float64
	ChargingPowerPhase    [3]float64
	ChargingCurrentPhase  [3]float64
	Frequency             float64
}

// NRGSettings is the /api/settings request/response
type NRGSettings struct {
	Info   NRGInfo `json:"omitempty"`
	Values NRGValues
}

// NRGInfo is NRGSettings.Info
type NRGInfo struct {
	Connected bool `json:"omitempty"`
}

// NRGValues is NRGSettings.Values
type NRGValues struct {
	ChargingStatus  NRGChargingStatus
	ChargingCurrent NRGChargingCurrent
	DeviceMetadata  NRGDeviceMetadata
}

// NRGChargingStatus is NRGSettings.Values.ChargingStatus
type NRGChargingStatus struct {
	Charging *bool `json:"omitempty"` // use pointer to allow omitting false
}

// NRGChargingCurrent is NRGSettings.Values.ChargingCurrent
type NRGChargingCurrent struct {
	Value float64 `json:"omitempty"`
}

// NRGDeviceMetadata is NRGSettings.Values.DeviceMetadata
type NRGDeviceMetadata struct {
	Password string
}

// NRGKickConnect charger implementation
type NRGKickConnect struct {
	*request.Helper
	log      *util.Logger
	uri      string
	mac      string
	password string
}

func init() {
	registry.Add("nrgkick-connect", NewNRGKickConnectFromConfig)
}

// NewNRGKickConnectFromConfig creates a NRGKickConnect charger from generic config
func NewNRGKickConnectFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		URI, Mac, Password string `validate:"required"`
	}{}
	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	return NewNRGKickConnect(cc.URI, cc.Mac, cc.Password)
}

// NewNRGKickConnect creates NRGKickConnect charger
func NewNRGKickConnect(uri, mac, password string) (*NRGKickConnect, error) {
	log := util.NewLogger("nrgconn")
	nrg := &NRGKickConnect{
		log:      log,
		Helper:   request.NewHelper(log),
		uri:      uri,
		mac:      mac,
		password: password,
	}

	nrg.log.WARN.Println("-- experimental --")

	return nrg, nil
}

func (nrg *NRGKickConnect) apiURL(api apiFunction) string {
	return fmt.Sprintf("%s/api/%s/%s", nrg.uri, api, nrg.mac)
}

func (nrg *NRGKickConnect) getJSON(url string, result interface{}) error {
	err := nrg.GetJSON(url, &result)
	if err != nil {
		var res NRGResponse
		if resp := nrg.LastResponse(); resp != nil {
			_ = request.DecodeJSON(resp, &res)
		}

		return fmt.Errorf("response: %s", res.Message)
	}

	return err
}

func (nrg *NRGKickConnect) putJSON(url string, data interface{}) error {
	var resp NRGResponse
	req, err := request.New(http.MethodPut, url, request.MarshalJSON(data))

	if err == nil {
		if err = nrg.DoJSON(req, &resp); err != nil {
			if resp.Message != "" {
				return fmt.Errorf("response: %s", resp.Message)
			}
		}
	}

	return err
}

// Status implements the Charger.Status interface
func (nrg *NRGKickConnect) Status() (api.ChargeStatus, error) {
	return api.StatusC, nil
}

// Enabled implements the Charger.Enabled interface
func (nrg *NRGKickConnect) Enabled() (bool, error) {
	var settings NRGSettings
	err := nrg.getJSON(nrg.apiURL(apiSettings), &settings)

	return *settings.Values.ChargingStatus.Charging, err
}

// Enable implements the Charger.Enable interface
func (nrg *NRGKickConnect) Enable(enable bool) error {
	settings := NRGSettings{}
	settings.Values.DeviceMetadata.Password = nrg.password
	settings.Values.ChargingStatus.Charging = &enable

	return nrg.putJSON(nrg.apiURL(apiSettings), settings)
}

// MaxCurrent implements the Charger.MaxCurrent interface
func (nrg *NRGKickConnect) MaxCurrent(current int64) error {
	settings := NRGSettings{}
	settings.Values.DeviceMetadata.Password = nrg.password
	settings.Values.ChargingCurrent.Value = float64(current)

	return nrg.putJSON(nrg.apiURL(apiSettings), settings)
}

// CurrentPower implements the Meter interface
func (nrg *NRGKickConnect) CurrentPower() (float64, error) {
	var measurements NRGMeasurements
	err := nrg.getJSON(nrg.apiURL(apiMeasurements), &measurements)

	return 1000 * measurements.ChargingPower, err
}

// TotalEnergy implements the MeterEnergy interface
func (nrg *NRGKickConnect) TotalEnergy() (float64, error) {
	var measurements NRGMeasurements
	err := nrg.getJSON(nrg.apiURL(apiMeasurements), &measurements)

	return measurements.ChargingEnergyOverAll, err
}

// Currents implements the MeterCurrent interface
func (nrg *NRGKickConnect) Currents() (float64, float64, float64, error) {
	var measurements NRGMeasurements
	err := nrg.getJSON(nrg.apiURL(apiMeasurements), &measurements)

	if len(measurements.ChargingCurrentPhase) != 3 {
		return 0, 0, 0, fmt.Errorf("unexpected response: %v", measurements)
	}

	return measurements.ChargingCurrentPhase[0],
		measurements.ChargingCurrentPhase[1],
		measurements.ChargingCurrentPhase[2],
		err
}

// ChargedEnergy implements the ChargeRater interface
// NOTE: apparently shows energy of a stopped charging session, hence substituted by TotalEnergy
// func (nrg *NRGKickConnect) ChargedEnergy() (float64, error) {
// 	var measurements NRGMeasurements
// 	err := nrg.getJSON(nrg.apiURL(apiMeasurements), &measurements)
// 	return measurements.ChargingEnergy, err
// }
