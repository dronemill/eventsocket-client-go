package eventsocketclient

import (
	"fmt"

	"github.com/nu7hatch/gouuid"
)

func makeUuid() string {
	uid, err := uuid.NewV4()
	if err != nil {
		panic(fmt.Sprintf("Error getting an uuid:", err))
	}

	return uid.String()
}
