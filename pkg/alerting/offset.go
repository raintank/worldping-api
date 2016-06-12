package alerting

import (
	"fmt"
	"strconv"

	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func LoadOrSetOffset() int {
	offset, err := sqlstore.GetAlertSchedulerValue("offset")
	if err != nil {
		log.Error(3, "failure querying for current offset: %q", err)
		return 30
	}
	if offset == "" {
		log.Debug("initializing offset to default value of 30 seconds.")
		setOffset(30)
		return 30
	}
	i, err := strconv.Atoi(offset)
	if err != nil {
		panic(fmt.Sprintf("failure reading in offset: %q. input value was: %q", err, offset))
	}
	return i
}

func setOffset(offset int) {
	err := sqlstore.UpdateAlertSchedulerValue("offset", fmt.Sprintf("%d", offset))
	if err != nil {
		log.Error(3, "Could not persist offset: %q", err)
	}
}
