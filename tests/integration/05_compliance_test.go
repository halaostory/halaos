package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSSSTable(t *testing.T) {
	resp, status, err := apiGet("/compliance/sss-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.GreaterOrEqual(t, len(list), 20, "SSS table should have 20+ brackets")
	t.Logf("SSS table: %d brackets", len(list))
}

func TestGetPhilHealthTable(t *testing.T) {
	resp, status, err := apiGet("/compliance/philhealth-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "PhilHealth table should have data")
	t.Logf("PhilHealth table: %d rows", len(list))
}

func TestGetPagIBIGTable(t *testing.T) {
	resp, status, err := apiGet("/compliance/pagibig-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "PagIBIG table should have data")
	t.Logf("PagIBIG table: %d rows", len(list))
}

func TestGetBIRTaxTable(t *testing.T) {
	// Test semi-monthly (default)
	resp, status, err := apiGet("/compliance/bir-tax-table", nil)
	require.NoError(t, err)
	requireSuccess(t, resp, status)

	list := extractList(t, resp)
	assert.NotEmpty(t, list, "BIR tax table should have brackets")
	t.Logf("BIR tax table (semi_monthly): %d brackets", len(list))

	// Test monthly frequency
	resp2, status2, err2 := apiGet("/compliance/bir-tax-table", map[string]string{"frequency": "monthly"})
	require.NoError(t, err2)
	requireSuccess(t, resp2, status2)

	list2 := extractList(t, resp2)
	assert.NotEmpty(t, list2, "BIR monthly tax table should have brackets")
	t.Logf("BIR tax table (monthly): %d brackets", len(list2))
}
