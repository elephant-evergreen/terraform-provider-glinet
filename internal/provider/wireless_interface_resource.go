package provider

import (
	"context"
	"fmt"
	"strconv"
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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var wirelessEncryptionModes = map[string]struct{}{
	"none": {}, "psk": {}, "psk2": {}, "psk-mixed": {},
	"sae": {}, "sae-mixed": {}, "wpa": {}, "wpa2": {},
	"wpa-mixed": {}, "owe": {},
}

var deprecatedWirelessEncryptionModes = map[string]struct{}{
	"psk": {}, "wpa": {}, "wpa2": {}, "wpa-mixed": {}, "owe": {},
}

func NewWirelessInterfaceResource() resource.Resource {
	return newTypedResource(typedWirelessInterfaceSpec())
}

func typedWirelessInterfaceSpec() typedSpec {
	return typedSpec{
		TypeName:    "wireless_interface",
		Description: "Manage one OpenWrt Wi-Fi access point. Keep a direct wired management path for changes. For adoption, omit `key_wo` and `key_version` to keep the current passphrase.",
		Attributes: map[string]schema.Attribute{
			"section": schema.StringAttribute{Required: true, MarkdownDescription: "UCI wifi-iface section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"ssid":    schema.StringAttribute{Required: true, MarkdownDescription: "SSID (1–32 UTF-8 bytes)."},
			"device":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Radio device, such as `mt798611`."},
			"network": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Network interface this AP joins."},
			"mode":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ap"), MarkdownDescription: "Wi-Fi mode, normally `ap`."},
			"encryption": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("psk2"),
				MarkdownDescription: "OpenWrt `encryption` mode. The stock Admin Panel exposes `none`, `psk2`, `psk-mixed`, `sae`, and `sae-mixed`. Other OpenWrt values remain accepted in 0.2.x for compatibility and will be restricted in the next minor release.",
				Validators:          []validator.String{wirelessEncryptionValidator{}},
			},
			"ieee80211w": schema.Int64Attribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Protected Management Frames: `0` disables, `1` makes PMF optional, and `2` requires it. When omitted, the provider derives `2` for `sae`, `1` for `sae-mixed`, and `0` otherwise.",
				Validators:          []validator.Int64{int64EnumValidator{allowed: []int64{0, 1, 2}}},
			},
			"wps_pushbutton": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "WPS push-button. When omitted, an existing section keeps its current value and a new section starts disabled. WPS is refused with WPA3 modes.",
			},
			"ifname":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional radio interface name."},
			"macaddr":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional MAC address override."},
			"wds":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable WDS."},
			"wpa_group_rekey": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "WPA group rekey interval in seconds; zero leaves it unset."},
			"hidden":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Hide the SSID."},
			"isolate":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Client isolation."},
			"disabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Disable the AP."},
			"key_wo":          schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Write-only WPA passphrase (8–63 printable ASCII). It is not stored in plan or state."},
			"key_version":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Client-managed key generation. Increase it when rotating `key_wo`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			section := stringOf(values, "section")
			if section == "" {
				return router.Section{}, fmt.Errorf("section is required")
			}
			encryption := defaultString(stringOf(values, "encryption"), "psk2")
			options := map[string]string{
				"disabled":   boolString(boolOf(values, "disabled", false)),
				"encryption": encryption, "hidden": boolString(boolOf(values, "hidden", false)), "isolate": boolString(boolOf(values, "isolate", true)),
			}
			pmf := defaultIEEE80211w(encryption)
			if value, ok := knownInt64(values, "ieee80211w"); ok {
				pmf = value
			}
			options["ieee80211w"] = strconv.FormatInt(pmf, 10)
			options["wds"] = boolString(boolOf(values, "wds", false))
			if rekey := int64Of(values, "wpa_group_rekey"); rekey > 0 {
				options["wpa_group_rekey"] = strconv.FormatInt(rekey, 10)
			}
			if value, ok := knownBool(values, "wps_pushbutton"); ok {
				options["wps_pushbutton"] = boolString(value)
			}
			ssid := stringOf(values, "ssid")
			options["ssid"] = ssid
			for _, field := range []struct{ attrName, option string }{{"device", "device"}, {"network", "network"}, {"mode", "mode"}, {"ifname", "ifname"}, {"macaddr", "macaddr"}} {
				value := stringOf(values, field.attrName)
				options[field.option] = value
			}
			return router.Section{Package: "wireless", Type: "wifi-iface", Name: section, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			encryption := defaultString(section.Options["encryption"], "psk2")
			pmf := defaultIEEE80211w(encryption)
			if value, ok := section.Options["ieee80211w"]; ok && value != "" {
				if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
					pmf = parsed
				}
			}
			return map[string]attr.Value{
				"section":         stringValue(section.Name),
				"ssid":            stringValue(section.Options["ssid"]),
				"device":          stringValue(section.Options["device"]),
				"network":         stringValue(section.Options["network"]),
				"mode":            stringValue(defaultString(section.Options["mode"], "ap")),
				"encryption":      stringValue(encryption),
				"ieee80211w":      int64Value(pmf),
				"wps_pushbutton":  boolValue(section.Options["wps_pushbutton"] == "1"),
				"ifname":          stringValue(section.Options["ifname"]),
				"macaddr":         stringValue(section.Options["macaddr"]),
				"wds":             boolValue(section.Options["wds"] == "1"),
				"wpa_group_rekey": int64Value(parseOptionInt64(section.Options["wpa_group_rekey"])),
				"hidden":          boolValue(section.Options["hidden"] == "1"),
				"isolate":         boolValue(section.Options["isolate"] != "0"),
				"disabled":        boolValue(section.Options["disabled"] == "1"),
				"key_wo":          types.StringNull(),
				"key_version":     int64Value(1),
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
			return validateWirelessSecurity(values)
		},
		ValidateConfig: validateWirelessSecurity,
		ModifyPlan:     modifyWirelessPlan,
		PreserveUnset:  []string{"ieee80211w", "macaddr", "wds", "wpa_group_rekey", "wps_pushbutton"},
	}
}

func validateWirelessSecurity(values map[string]attr.Value) error {
	encryption, ok := knownString(values, "encryption")
	if !ok || encryption == "" {
		return nil
	}
	if _, ok := wirelessEncryptionModes[encryption]; !ok {
		return fmt.Errorf("encryption must be one of none, psk, psk2, psk-mixed, sae, sae-mixed, wpa, wpa2, wpa-mixed, or owe")
	}
	pmf, pmfKnown := knownInt64(values, "ieee80211w")
	if pmfKnown {
		if pmf < 0 || pmf > 2 {
			return fmt.Errorf("ieee80211w must be 0, 1, or 2")
		}
		switch encryption {
		case "sae":
			if pmf != 2 {
				return fmt.Errorf("ieee80211w must be 2 when encryption is sae")
			}
		case "sae-mixed":
			if pmf == 0 {
				return fmt.Errorf("ieee80211w must be 1 or 2 when encryption is sae-mixed")
			}
		}
	}
	if wps, wpsKnown := knownBool(values, "wps_pushbutton"); wpsKnown && wps {
		switch encryption {
		case "psk", "psk2", "psk-mixed":
		default:
			return fmt.Errorf("wps_pushbutton requires a PSK mode")
		}
	}
	return nil
}

func modifyWirelessPlan(plan, config, state map[string]attr.Value, creating bool) (map[string]attr.Value, error) {
	encryption, encryptionKnown := knownString(plan, "encryption")
	if !encryptionKnown {
		encryption = "psk2"
	}
	stateEncryption, stateKnown := knownString(state, "encryption")
	encryptionChanged := creating || !stateKnown || stateEncryption != encryption
	if value, ok := config["ieee80211w"].(types.Int64); !ok || value.IsNull() {
		if _, known := knownInt64(plan, "ieee80211w"); !known || encryptionChanged {
			if encryptionKnown {
				plan["ieee80211w"] = int64Value(defaultIEEE80211w(encryption))
			} else {
				plan["ieee80211w"] = types.Int64Unknown()
			}
		}
	}
	if value, ok := config["wps_pushbutton"].(types.Bool); !ok || value.IsNull() {
		switch {
		case creating, encryption == "sae", encryption == "sae-mixed", encryption == "none":
			plan["wps_pushbutton"] = boolValue(false)
		default:
			if current, ok := plan["wps_pushbutton"].(types.Bool); !ok || current.IsNull() || current.IsUnknown() {
				plan["wps_pushbutton"] = boolValue(false)
			}
		}
	}
	return plan, nil
}

func defaultIEEE80211w(encryption string) int64 {
	switch encryption {
	case "sae":
		return 2
	case "sae-mixed":
		return 1
	default:
		return 0
	}
}

type wirelessEncryptionValidator struct{}

func (wirelessEncryptionValidator) Description(context.Context) string {
	return "value must be a supported OpenWrt wireless encryption mode"
}

func (wirelessEncryptionValidator) MarkdownDescription(context.Context) string {
	return "value must be a supported OpenWrt wireless encryption mode"
}

func (v wirelessEncryptionValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if _, ok := wirelessEncryptionModes[value]; !ok {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid encryption", "encryption must be one of none, psk, psk2, psk-mixed, sae, sae-mixed, wpa, wpa2, wpa-mixed, or owe.")
		return
	}
	if _, ok := deprecatedWirelessEncryptionModes[value]; ok {
		resp.Diagnostics.AddAttributeWarning(req.Path, "Encryption mode will be removed", "This encryption mode is accepted in 0.2.x for compatibility and will be rejected in the next minor release. Use none, psk2, psk-mixed, sae, or sae-mixed.")
	}
}

type int64EnumValidator struct {
	allowed []int64
}

func (v int64EnumValidator) Description(context.Context) string {
	return "value must be one of the allowed values"
}

func (v int64EnumValidator) MarkdownDescription(context.Context) string {
	return "value must be one of the allowed values"
}

func (v int64EnumValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueInt64()
	for _, allowed := range v.allowed {
		if value == allowed {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", "value must be one of 0, 1, or 2.")
}
