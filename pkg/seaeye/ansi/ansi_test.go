package ansi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToHTML(t *testing.T) {
	assert.Equal(t, []byte(""), ToHTML([]byte("")))
}
