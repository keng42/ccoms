package info

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNewerVersion(t *testing.T) {
	isNewer, err := IsNewerVersion("", "", "", "")
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalid, err)
	assert.Equal(t, false, isNewer)

	isNewer, err = IsNewerVersion("0.1.2", "1", "0.1.1", "1")
	assert.Nil(t, err)
	assert.Equal(t, true, isNewer)

	isNewer, err = IsNewerVersion("0.4.2", "1", "0.11.1", "1")
	assert.Nil(t, err)
	assert.Equal(t, false, isNewer)

	isNewer, err = IsNewerVersion("0.1.2", "4", "0.1.2", "3")
	assert.Nil(t, err)
	assert.Equal(t, true, isNewer)

	isNewer, err = IsNewerVersion("0.1.2", "1", "0.1.2", "3")
	assert.Nil(t, err)
	assert.Equal(t, false, isNewer)
}
