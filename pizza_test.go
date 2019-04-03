package pizza

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLiveClient() *Client {
	return &Client{
		Email:     "foo@example.com",
		FirstName: "Dominos",
		LastName:  "Pizza",
		Phone:     "443-554-6667",

		Debug:      false,
		HTTPClient: http.DefaultClient,
		BaseURL:    mustParse("https://order.dominos.com/power/"),
	}
}

func TestGetDeliveryStores(t *testing.T) {
	addr := Address{
		Street:     "5905 Bonnie View Dr",
		City:       "Baltimore",
		Region:     "MD",
		PostalCode: "21209",
		Type:       "Delivery",
	}

	cli := newLiveClient()

	stores, err := cli.GetDeliveryStores(addr)
	require.NoError(t, err)
	assert.Equal(t, 0, len(stores))

	addr = Address{
		Street:     "747 W 40th St",
		City:       "Baltimore",
		Region:     "MD",
		PostalCode: "21211",
		Type:       "House",
	}

	stores, err = cli.GetDeliveryStores(addr)
	require.NoError(t, err)
	assert.Equal(t, "4626", stores[0].ID)
}

func TestGetMenuItems(t *testing.T) {
	cli := newLiveClient()

	items, err := cli.GetMenuItems("4626")
	require.NoError(t, err)
	assert.NotNil(t, items)
}

func TestPriceOrder(t *testing.T) {
	cli := newLiveClient()
	addr := Address{
		Street:     "747 W 40th St",
		City:       "Baltimore",
		Region:     "MD",
		PostalCode: "21211",
		Type:       "House",
	}
	price, err := cli.PriceOrder("4626", addr, map[string]int{"P14IRECZ": 1})
	require.NoError(t, err)
	assert.Equal(t, 299, price.DeliveryCents)
	assert.Equal(t, 108, price.TaxCents)
	assert.Equal(t, 2206, price.CustomerCents)
}
