package info

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

var (
	Version    = "0.0.0"
	Dist       = "1"
	GitRev     = "000000"
	BuildTime  = "2000-01-01_00:00:00"
	InstanceID = uuid.New().String()
)

var ErrInvalid = errors.New("invalid version")

var (
	EnvMode  = "development"
	EnvColor = false
)

func init() {
	mode := os.Getenv("CCOMS_MODE")
	if mode != "" {
		EnvMode = mode
	}

	color := os.Getenv("CCOMS_COLOR")
	EnvColor = color != "" && color != "false" && color != "0"
}

// return A is newer than B
func IsNewerVersion(verA, distA, verB, distB string) (bool, error) {
	aa := strings.Split(verA, ".")
	bb := strings.Split(verB, ".")
	if len(aa) != 3 || len(bb) != 3 {
		return false, ErrInvalid
	}

	aa0 := parseInt64(aa[0])
	aa1 := parseInt64(aa[1])
	aa2 := parseInt64(aa[2])
	bb0 := parseInt64(bb[0])
	bb1 := parseInt64(bb[1])
	bb2 := parseInt64(bb[2])
	da := parseInt64(distA)
	db := parseInt64(distB)

	if aa0 != bb0 {
		return aa0 > bb0, nil
	}
	if aa1 != bb1 {
		return aa1 > bb1, nil
	}
	if aa2 != bb2 {
		return aa2 > bb2, nil
	}
	return da > db, nil
}

func parseInt64(str string) int64 {
	res, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}
	return res
}
