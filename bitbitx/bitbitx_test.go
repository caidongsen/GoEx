package bitbitx

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	DES3Key = "des3keys0123456701234567"
)

var bbx *BitBitx

func init() {
	bbx = New(http.DefaultClient, DES3Key)
}

func TestRegister(t *testing.T) {
	require := require.New(t)
	mobile := "13858075274"
	email := "hxz@disanbo.com"
	name := "hxzqlh"
	pwd := "123456"
	payPwd := "823882"
	err := bbx.Register(mobile, email, name, pwd, payPwd)
	require.Nil(err)
}

func TestLogin(t *testing.T) {
	require := require.New(t)
	name := "hxz@disanbo.com"
	pwd := "123456"
	err := bbx.Login(name, pwd)
	require.Nil(err)
}
