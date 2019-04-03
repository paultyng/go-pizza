package pizza

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func stringNumberToCents(s string) (int, error) {
	raw := strings.Replace(s, ".", "", 1)
	return strconv.Atoi(raw)
}

type Client struct {
	HTTPClient *http.Client
	BaseURL    *url.URL
	Debug      bool

	Email     string
	FirstName string
	LastName  string
	Phone     string
}

func (c *Client) do(method string, reqURL *url.URL, requestBody interface{}, responseBody interface{}) error {
	reqURL = c.BaseURL.ResolveReference(reqURL)

	var reqBuffer io.Reader
	hasRequestBody := (requestBody != nil)
	if hasRequestBody {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		reqBuffer = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, reqURL.String(), reqBuffer)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", "https://order.dominos.com/en/pages/order/")
	if hasRequestBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Debug {
		dump, _ := httputil.DumpRequest(req, true)
		log.Println(string(dump))
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	if c.Debug {
		dump, _ := httputil.DumpResponse(resp, true)
		log.Println(string(dump))
	}
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respBytes, responseBody)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetDeliveryStores(address Address) ([]Store, error) {
	reqURL := mustParse("store-locator")
	qs := reqURL.Query()
	qs.Set("s", address.Street)
	qs.Set("c", fmt.Sprintf("%s, %s %s", address.City, address.Region, address.PostalCode))
	qs.Add("s", "Delivery")
	reqURL.RawQuery = qs.Encode()

	var resp struct {
		Stores []struct {
			StoreID         string
			IsDeliveryStore bool

			ServiceMethodEstimatedWaitMinutes struct {
				Delivery struct {
					Min int
				}
			}
		}
	}

	err := c.do("GET", reqURL, nil, &resp)
	if err != nil {
		return nil, err
	}

	stores := make([]Store, 0, len(resp.Stores))
	for _, s := range resp.Stores {
		if s.IsDeliveryStore {
			stores = append(stores, Store{
				ID:              s.StoreID,
				DeliveryMinutes: s.ServiceMethodEstimatedWaitMinutes.Delivery.Min,
			})
		}
	}

	return stores, nil
}

func (c *Client) GetMenuItems(storeID string) ([]MenuItem, error) {
	reqURL := mustParse(fmt.Sprintf("store/%s/menu?lang=en&structured=true", storeID))

	var resp struct {
		Variants map[string]struct {
			Code  string
			Price string
			Name  string
		}
	}

	err := c.do("GET", reqURL, nil, &resp)
	if err != nil {
		return nil, err
	}

	items := make([]MenuItem, 0, len(resp.Variants))
	for _, v := range resp.Variants {
		price, err := stringNumberToCents(v.Price)
		if err != nil {
			return nil, err
		}
		items = append(items, MenuItem{
			Code:       v.Code,
			Name:       v.Name,
			PriceCents: price,
		})
	}
	return items, nil
}

func (c *Client) PriceOrder(storeID string, address Address, products map[string]int) (*OrderPrice, error) {
	reqURL := mustParse("price-order")

	reqProducts := []map[string]interface{}{}
	for code, qty := range products {
		reqProducts = append(reqProducts, map[string]interface{}{
			"Code":       code,
			"ID":         1,
			"isNew":      true,
			"Qty":        qty,
			"AutoRemove": false,
		})
	}

	req := map[string]map[string]interface{}{
		"Order": {
			"Address":               address,
			"Coupons":               []string{},
			"CustomerID":            "",
			"Extension":             "",
			"OrderChannel":          "OLO",
			"OrderID":               "",
			"NoCombine":             true,
			"OrderMethod":           "Web",
			"OrderTaker":            nil,
			"Payments":              []map[string]interface{}{{"Type": "Cash"}},
			"Products":              reqProducts,
			"Market":                "",
			"Currency":              "",
			"ServiceMethod":         "Delivery",
			"Tags":                  map[string]string{},
			"Version":               "1.0",
			"SourceOrganizationURI": "order.dominos.com",
			"LanguageCode":          "en",
			"Partners":              map[string]string{},
			"NewUser":               true,
			"metaData":              map[string]string{},
			"Amounts":               map[string]string{},
			"BusinessDate":          "",
			"EstimatedWaitMinutes":  "",
			"PriceOrderTime":        "",
			"AmountBreakdown":       map[string]string{},
			"StoreID":               storeID,

			"Email":     c.Email,
			"FirstName": c.FirstName,
			"LastName":  c.LastName,
			"Phone":     c.Phone,
		},
	}

	var resp struct {
		Status json.Number
		Order  struct {
			OrderID          string
			AmountsBreakdown struct {
				DeliveryFee string
				Tax         json.Number
				Customer    json.Number
			}
		}
	}

	err := c.do("POST", reqURL, req, &resp)
	if err != nil {
		return nil, err
	}

	status, err := resp.Status.Int64()
	if err != nil {
		return nil, err
	}
	if status == -1 {
		return nil, errors.New("dominos does not like this order for some reason")
	}

	delivery, err := stringNumberToCents(resp.Order.AmountsBreakdown.DeliveryFee)
	if err != nil {
		return nil, err
	}

	tax, err := stringNumberToCents(string(resp.Order.AmountsBreakdown.Tax))
	if err != nil {
		return nil, err
	}

	customer, err := stringNumberToCents(string(resp.Order.AmountsBreakdown.Customer))
	if err != nil {
		return nil, err
	}

	return &OrderPrice{
		ID:            resp.Order.OrderID,
		DeliveryCents: delivery,
		TaxCents:      tax,
		CustomerCents: customer,
	}, nil

}

type OrderPrice struct {
	ID string

	DeliveryCents int
	TaxCents      int
	CustomerCents int
}

type Address struct {
	Street     string
	City       string
	Region     string
	PostalCode string
	Type       string
}

type Store struct {
	ID              string
	DeliveryMinutes int
}

type MenuItem struct {
	Code       string
	Name       string
	PriceCents int
}
