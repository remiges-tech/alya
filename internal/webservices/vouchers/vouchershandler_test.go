package voucher

import (
	"encoding/json"
	"fmt"
	"go-framework/internal/pg"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/logharbour"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Run the tests
	os.Exit(m.Run())
}

func TestCreateVoucher(t *testing.T) {

	compKeys := []string{"employee_id", "amount"}

	// Switch to test mode so you don't get such noisy output
	gin.SetMode(gin.TestMode)
	// Setup your router, just like we did in main
	r := gin.Default()
	pg := pg.Connect()
	sqlq := sqlc.New(pg)
	lh := logharbour.New()
	voucherHandler := NewHandler(sqlq, lh)
	voucherHandler.RegisterVoucherHandlers(r)

	//=============== INSERTING A NEW RECORDS =================//
	// Create a request to send to the above route
	var jsonStr = `{
					"ver": 1,
					"authtoken":"test",
					"data": {
						"employee_id": 1,
						"date_of_claim": "2023-01-21",
						"amount": 855.25,
						"description": "desc-test-3"
					}
				}`

	// Voucher insert request
	req, _ := http.NewRequest(http.MethodPost, "/voucher", strings.NewReader(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder to inspect the response
	w := httptest.NewRecorder()

	// Perform the request
	r.ServeHTTP(w, req)

	// Check the status code is what you expect
	if w.Code != http.StatusOK {
		t.Errorf("Expected to get status %d but instead got %d. Body: %s\n", http.StatusOK, w.Code, w.Body.String())
	}

	//=============== FETCHING ThE INSERTED RECORDS =================//

	// voucher fetch request for above inserted records
	voucherID := 63 // specify PK of the entity to fetch the record
	req1, _ := http.NewRequest(http.MethodGet, "/voucher/"+strconv.Itoa(voucherID), nil)
	req1.Header.Set("Content-Type", "application/json")
	// Create a response recorder so you can inspect the response
	w1 := httptest.NewRecorder()

	// Perform the request
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected to get status %d but instead got %d. Body: %s\n", http.StatusOK, w1.Code, w1.Body.String())
	}

	// Excepect true if post data matched get data based on compKeys
	compResp := compareData(t, jsonStr, w1.Body.String(), compKeys)
	if compResp == false {
		t.Errorf("Insertion failed\n")
	} else {
		t.Logf("Insertion passed\n")
	}

}

// compare 2 JSON string object after parsing
func compareData(t *testing.T, data1 string, data2 string, compKeys []string) bool {

	var data1Struct map[string]interface{}
	err := json.Unmarshal([]byte(data1), &data1Struct)
	if err != nil {
		t.Errorf("Failed to decode response for data1: %s\n", err.Error())
		return false
	}

	var data2Struct map[string]interface{}
	err = json.Unmarshal([]byte(data2), &data2Struct)
	if err != nil {
		t.Errorf("Failed to decode response for data2: %s\n", err.Error())
		return false
	}

	fmt.Println(compKeys)
	for _, k := range compKeys {
		if data1Struct["data"].(map[string]interface{})[k] != data2Struct["data"].(map[string]interface{})[k] {
			// fmt.Printf("%T:", data1Struct["data"].(map[string]interface{})[k])
			// fmt.Printf("%T:", data2Struct["data"].(map[string]interface{})[k])
			return false
		}
	}
	return true
}
