package provider

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type configurationRunner struct {
	mu     sync.Mutex
	actual router.Configuration
	writes int
}

func (r *configurationRunner) Run(_ context.Context, script string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.Contains(script, "io.write(json.encode(result))") {
		return json.Marshal(r.actual)
	}
	if strings.Contains(script, "assert(uci:save") {
		match := regexp.MustCompile(`local spec=json.decode\('((?:\\.|[^'])*)'\)`).FindStringSubmatch(script)
		if len(match) != 2 {
			panic("Missing fixture configuration")
		}
		data := strings.NewReplacer(`\\`, `\`, `\'`, `'`).Replace(match[1])
		if json.Unmarshal([]byte(data), &r.actual) != nil {
			panic("Invalid fixture configuration")
		}
		r.writes++
	}
	return nil, nil
}
func TestConfigurationProtocolAdoptionUpdateDriftAndForget(t *testing.T) {
	initial := router.Configuration{{Package: "firewall", Name: "example", Type: "rule", Options: map[string]string{"name": "Example", "target": "REJECT"}}}
	remote := &configurationRunner{actual: initial}
	client := router.NewClient(remote)
	factories := map[string]func() (tfprotov6.ProviderServer, error){"glinet": providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})}
	config := func(label string) string {
		return `resource "glinet_uci_configuration" "test" {
 configuration=jsonencode([{package="firewall",name="example",type="rule",options={name="` + label + `",target="REJECT"}}])
 wireless_keys_wo={}
 wireless_keys_version=1
}`
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: factories,
		CheckDestroy: func(*terraform.State) error {
			remote.mu.Lock()
			defer remote.mu.Unlock()
			if remote.writes != 2 || remote.actual[0].Options["name"] != "Updated" {
				t.Fatalf("Unexpected writes or destroy mutation: %d", remote.writes)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config("Example"), Check: func(*terraform.State) error {
				if remote.writes != 0 {
					t.Fatal("Adoption mutated the router")
				}
				return nil
			}},
			{Config: config("Updated")},
			{PreConfig: func() { remote.mu.Lock(); remote.actual[0].Options["name"] = "Outside"; remote.mu.Unlock() }, Config: config("Updated"), PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: config("Updated")},
		}})
}
