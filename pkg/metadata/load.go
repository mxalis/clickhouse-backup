package metadata

import (
	"encoding/json"
	apexLog "github.com/apex/log"
	"io/ioutil"
)

func (tm *TableMetadata) Load(location string) (uint64, error) {
	data, err := ioutil.ReadFile(location)
	if err != nil {
		return 0, err
	}
	if err := json.Unmarshal(data, tm); err != nil {
		return 0, err
	}
	apexLog.Debugf("success TableMedata.Load(%s)", location)
	return uint64(len(data)), nil
}
