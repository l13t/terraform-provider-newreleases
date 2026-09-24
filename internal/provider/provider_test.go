package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
	"github.com/l13t/terraform-provider-newreleases/internal/fakeapi"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"newreleases": providerserver.NewProtocol6WithError(New("test")()),
}

// TestMain points the provider at an in-memory fake API when
// NEWRELEASES_FAKE=1 (task testacc). Otherwise acceptance tests talk to the
// API selected by NEWRELEASES_API_KEY and NEWRELEASES_BASE_URL
// (task testacc:live). The provider runs in-process, so environment variables
// set here reach its Configure.
func TestMain(m *testing.M) {
	if os.Getenv("NEWRELEASES_FAKE") != "1" {
		os.Exit(m.Run())
	}
	fake := fakeapi.New()
	os.Setenv("NEWRELEASES_API_KEY", fakeapi.APIKey)
	os.Setenv("NEWRELEASES_BASE_URL", fake.URL+"/")
	code := m.Run()
	fake.Close()
	os.Exit(code)
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("NEWRELEASES_API_KEY") == "" {
		t.Fatal("acceptance tests need NEWRELEASES_FAKE=1 (fake API) or NEWRELEASES_API_KEY (live API)")
	}
}

// testAccClient returns an API client for out-of-band existence and
// destruction checks.
func testAccClient(t *testing.T) *newreleases.Client {
	t.Helper()
	c, err := client.New(client.Config{
		APIKey:  os.Getenv("NEWRELEASES_API_KEY"),
		BaseURL: os.Getenv("NEWRELEASES_BASE_URL"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// stateResourceAtAddress finds a managed resource in the root module of a
// state returned by `terraform show -json`.
func stateResourceAtAddress(state *tfjson.State, address string) (*tfjson.StateResource, error) {
	if state == nil || state.Values == nil || state.Values.RootModule == nil {
		return nil, fmt.Errorf("state has no root module")
	}
	for _, r := range state.Values.RootModule.Resources {
		if r.Address == address {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource %s not found in state", address)
}

func stateResourceID(state *tfjson.State, address string) (string, error) {
	r, err := stateResourceAtAddress(state, address)
	if err != nil {
		return "", err
	}
	id, ok := r.AttributeValues["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("resource %s has no id", address)
	}
	return id, nil
}

// stateCheckFunc adapts a function to statecheck.StateCheck.
type stateCheckFunc func(ctx context.Context, state *tfjson.State) error

func (f stateCheckFunc) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	resp.Error = f(ctx, req.State)
}
