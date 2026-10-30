package kafka

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateMessageEncodesSchemaID(t *testing.T) {
	message := []byte("hello")

	got, err := CreateMessage(message, 42)
	require.NoError(t, err)
	require.Equal(t, magicByte, got[0])
	require.Equal(t, uint32(42), binary.BigEndian.Uint32(got[1:5]))
	require.Equal(t, message, got[5:])
}

func TestCreateMessageRejectsNegativeSchemaID(t *testing.T) {
	_, err := CreateMessage([]byte("hello"), -1)
	require.Error(t, err)
}

func TestCreateMessageRejectsSchemaIDBeyondUint32(t *testing.T) {
	big := int64(math.MaxUint32) + 1
	if int64(int(big)) != big {
		t.Skip("int is 32 bits wide here: no schema ID can exceed uint32")
	}
	_, err := CreateMessage([]byte("hello"), int(big))
	require.Error(t, err)
}
