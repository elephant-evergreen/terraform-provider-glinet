package provider

import (
	"fmt"
	"strings"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewWirelessInterfaceResource() resource.Resource {
	return newTypedResource(typedWirelessInterfaceSpec())
}

func typedWirelessInterfaceSpec() typedSpec {
	return typedSpec{
		TypeName:    "wireless_interface",
		Description: "Manage one OpenWrt `wireless` wifi-iface section. Wi-Fi passphrases use a write-only argument and never enter plan or state; omit it to preserve the existing key.",
		Attributes: map[string]schema.Attribute{
			"section":     schema.StringAttribute{Required: true, MarkdownDescription: "UCI wifi-iface section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"ssid":        schema.StringAttribute{Required: true, MarkdownDescription: "SSID (1–32 UTF-8 bytes)."},
			"device":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Radio device, such as `mt798611`."},
			"network":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Network interface this AP joins."},
			"mode":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ap"), MarkdownDescription: "Wi-Fi mode, normally `ap`."},
			"encryption":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("psk2"), MarkdownDescription: "Encryption, such as `psk2`."},
			"ifname":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional radio interface name."},
			"hidden":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Hide the SSID."},
			"isolate":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Client isolation."},
			"disabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Disable the AP."},
			"key_wo":      schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Write-only WPA passphrase (8–63 printable ASCII). It is not stored in plan or state."},
			"key_version": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Client-managed key generation. Increase it when rotating `key_wo`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			section := stringOf(values, "section")
			if section == "" {
				return router.Section{}, fmt.Errorf("section is required")
			}
			options := map[string]string{
				"disabled":   boolString(boolOf(values, "disabled", false)),
				"encryption": stringOf(values, "encryption"), "hidden": boolString(boolOf(values, "hidden", false)), "isolate": boolString(boolOf(values, "isolate", true)),
			}
			ssid := stringOf(values, "ssid")
			options["ssid"] = ssid
			for _, field := range []struct{ attrName, option string }{{"device", "device"}, {"network", "network"}, {"mode", "mode"}, {"ifname", "ifname"}} {
				value := stringOf(values, field.attrName)
				options[field.option] = value
			}
			return router.Section{Package: "wireless", Type: "wifi-iface", Name: section, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"section":     stringValue(section.Name),
				"ssid":        stringValue(section.Options["ssid"]),
				"device":      stringValue(section.Options["device"]),
				"network":     stringValue(section.Options["network"]),
				"mode":        stringValue(defaultString(section.Options["mode"], "ap")),
				"encryption":  stringValue(defaultString(section.Options["encryption"], "psk2")),
				"ifname":      stringValue(section.Options["ifname"]),
				"hidden":      boolValue(section.Options["hidden"] == "1"),
				"isolate":     boolValue(section.Options["isolate"] != "0"),
				"disabled":    boolValue(section.Options["disabled"] == "1"),
				"key_wo":      types.StringNull(),
				"key_version": int64Value(1),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a wifi-iface section name")
			}
			return map[string]attr.Value{"section": stringValue(id), "key_version": int64Value(1)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "section"), nil },
		Secrets: func(values map[string]attr.Value) map[string]string {
			key := stringOf(values, "key_wo")
			section := stringOf(values, "section")
			if key == "" || section == "" {
				return nil
			}
			return map[string]string{"wireless." + section + ".key": key}
		},
		WriteOnly: []string{"key_wo"},
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "section") == "" {
				return fmt.Errorf("section is required")
			}
			ssid := stringOf(values, "ssid")
			if !boolOf(values, "disabled", false) {
				if len(ssid) < 1 || len(ssid) > 32 || strings.ContainsFunc(ssid, func(r rune) bool { return r < 32 || r == 127 }) {
					return fmt.Errorf("enabled Wi-Fi sections require an ssid of 1–32 UTF-8 bytes without control characters")
				}
			}
			if key := stringOf(values, "key_wo"); key != "" {
				if len(key) < 8 || len(key) > 63 || strings.ContainsFunc(key, func(r rune) bool { return r < 32 || r > 126 }) {
					return fmt.Errorf("key_wo must contain 8–63 printable ASCII characters")
				}
			}
			switch stringOf(values, "mode") {
			case "ap", "sta", "mesh":
			default:
				return fmt.Errorf("mode must be ap, sta, or mesh")
			}
			return nil
		},
	}
}
