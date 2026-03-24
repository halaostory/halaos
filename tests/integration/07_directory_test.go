package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDirectory(t *testing.T) {
	resp, status, err := apiGet("/directory", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "directory should contain employees")
	t.Logf("Directory: %d entries", len(list))
}

func TestGetOrgChart(t *testing.T) {
	resp, status, err := apiGet("/directory/org-chart", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)
	t.Logf("Org chart: %d bytes", len(resp))
}
