package constantclient

import (
	"testing"

	"github.com/joeqian10/neo3-gogogo/rpc/models"
)

func TestAssertion(t *testing.T) {
	var a interface{} = (*models.RpcApplicationLog)(nil)
	_ = a.(*models.RpcApplicationLog)
}
