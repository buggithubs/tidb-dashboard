package datatype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntJSON(t *testing.T) {
	nv := Int(123)
	v, err := json.Marshal(nv)
	require.Nil(t, err)
	require.Equal(t, string(v), "123")

	st := struct {
		Foo Int
	}{
		Foo: Int(415425),
	}
	v, err = json.Marshal(st)
	require.Nil(t, err)
	require.Equal(t, string(v), `{"Foo":415425}`)

	var nv2 Int
	err = json.Unmarshal([]byte("12345"), &nv2)
	require.Nil(t, err)
	require.Equal(t, Int(12345), nv2)

	err = json.Unmarshal([]byte(`{"Foo":56789}`), &st)
	require.Nil(t, err)
	require.Equal(t, Int(56789), st.Foo)

	err = json.Unmarshal([]byte(`{"Foo":"123"}`), &st)
	require.NotNil(t, err)

	err = json.Unmarshal([]byte(`{"Foo":1300.45}`), &st)
	require.NotNil(t, err)

	nv3 := Int(48691071)
	v, err = json.Marshal(nv3)
	require.Nil(t, err)
	err = json.Unmarshal(v, &nv2)
	require.Nil(t, err)
	require.Equal(t, nv2, nv3)
}
