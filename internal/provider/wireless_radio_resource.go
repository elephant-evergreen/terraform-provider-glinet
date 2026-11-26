package provider

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var wirelessRadioModes = map[string]struct{}{
	"HT20": {}, "HT40": {}, "VHT20": {}, "VHT40": {}, "VHT80": {}, "VHT160": {},
	"HE20": {}, "HE40": {}, "HE80": {}, "HE160": {},
}

var wirelessRadio2gModes = map[string]struct{}{
	"HT20": {}, "HT40": {}, "HE20": {}, "HE40": {},
}

var wirelessRadio5gModes = map[string]struct{}{
	"VHT20": {}, "VHT40": {}, "VHT80": {}, "VHT160": {},
	"HE20": {}, "HE40": {}, "HE80": {}, "HE160": {},
}

var wirelessCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

func NewWirelessRadioResource() resource.Resource {
	return newTypedResource(typedWirelessRadioSpec())
}

func typedWirelessRadioSpec() typedSpec {
	return typedSpec{
		TypeName:                  "wireless_radio",
		Description:               "Manage one OpenWrt `wifi-device` radio. The resource adopts an existing radio section; it never creates or deletes radio hardware. Keep a direct wired management path for channel or width changes.",
		RequireExisting:           true,
		NoDelete:                  true,
		DeleteBehaviorDescription: "Destroy behavior. This resource preserves the adopted `wifi-device` section; `forget` is required and `delete` is rejected.",
		Attributes: map[string]schema.Attribute{
			"device":       schema.StringAttribute{Required: true, MarkdownDescription: "UCI `wifi-device` section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"band":         schema.StringAttribute{Computed: true, MarkdownDescription: "Radio band reported by the router, such as `2g` or `5g`."},
			"channel":      schema.StringAttribute{Required: true, MarkdownDescription: "Channel number or `auto`. The router's regulatory domain remains authoritative."},
			"htmode":       schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Channel width and generation, such as `HE40` or `HE80`. Accepted values are `HT20`, `HT40`, `VHT20`, `VHT40`, `VHT80`, `VHT160`, `HE20`, `HE40`, `HE80`, and `HE160`."},
			"txpower":      schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Transmit power percentage, 0–100."},
			"country":      schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Two-letter regulatory country code, such as `JP`. When omitted, the current value is preserved."},
			"disabled":     schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Disable the radio."},
			"noscan":       schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Do not scan for overlapping channels."},
			"legacy_rates": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Enable legacy data rates."},
			"beacon_int":   schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Beacon interval in milliseconds."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			device := stringOf(values, "device")
			if device == "" {
				return router.Section{}, fmt.Errorf("device is required")
			}
			options := map[string]string{}
			for _, name := range []string{"channel", "htmode", "country"} {
				if value, ok := knownString(values, name); ok && value != "" {
					options[name] = value
				}
			}
			if value, ok := knownInt64(values, "txpower"); ok {
				options["txpower"] = strconv.FormatInt(value, 10)
			}
			if value, ok := knownBool(values, "disabled"); ok {
				options["disabled"] = boolString(value)
			}
			for _, name := range []string{"noscan", "legacy_rates"} {
				if value, ok := knownBool(values, name); ok {
					options[name] = boolString(value)
				}
			}
			return router.Section{Package: "wireless", Type: "wifi-device", Name: device, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			txpower, _ := strconv.ParseInt(defaultString(section.Options["txpower"], "100"), 10, 64)
			return map[string]attr.Value{
				"device":       stringValue(section.Name),
				"band":         stringValue(section.Options["band"]),
				"channel":      stringValue(defaultString(section.Options["channel"], "auto")),
				"htmode":       stringValue(section.Options["htmode"]),
				"txpower":      int64Value(txpower),
				"country":      stringValue(section.Options["country"]),
				"disabled":     boolValue(section.Options["disabled"] == "1"),
				"noscan":       boolValue(section.Options["noscan"] == "1"),
				"legacy_rates": boolValue(section.Options["legacy_rates"] == "1"),
				"beacon_int":   int64Value(parseOptionInt64(section.Options["beacon_int"])),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a wifi-device section name")
			}
			return map[string]attr.Value{"device": stringValue(id)}, nil
		},
		ID:             func(values map[string]attr.Value) (string, error) { return stringOf(values, "device"), nil },
		Validate:       validateWirelessRadio,
		ValidateConfig: validateWirelessRadio,
	}
}

func validateWirelessRadio(values map[string]attr.Value) error {
	if stringOf(values, "device") == "" {
		return fmt.Errorf("device is required")
	}
	band := stringOf(values, "band")
	channel := stringOf(values, "channel")
	if channel == "" {
		return fmt.Errorf("channel is required")
	}
	if channel != "auto" {
		number, err := strconv.Atoi(channel)
		if err != nil || number < 1 || number > 196 {
			return fmt.Errorf("channel must be auto or a number between 1 and 196")
		}
		switch band {
		case "2g":
			if number > 14 {
				return fmt.Errorf("2g channels must be between 1 and 14")
			}
		case "5g":
			if number < 36 {
				return fmt.Errorf("5g channels must be between 36 and 196")
			}
		}
	}
	htmode := stringOf(values, "htmode")
	if htmode != "" {
		if _, ok := wirelessRadioModes[htmode]; !ok {
			return fmt.Errorf("htmode must be one of HT20, HT40, VHT20, VHT40, VHT80, VHT160, HE20, HE40, HE80, or HE160")
		}
		switch band {
		case "2g":
			if _, ok := wirelessRadio2gModes[htmode]; !ok {
				return fmt.Errorf("htmode %s is not supported on the 2g band", htmode)
			}
		case "5g":
			if _, ok := wirelessRadio5gModes[htmode]; !ok {
				return fmt.Errorf("htmode %s is not supported on the 5g band", htmode)
			}
		}
	}
	if txpower, ok := knownInt64(values, "txpower"); ok && (txpower < 0 || txpower > 100) {
		return fmt.Errorf("txpower must be between 0 and 100")
	}
	if country := stringOf(values, "country"); country != "" && !wirelessCountryPattern.MatchString(country) {
		return fmt.Errorf("country must be a two-letter uppercase code")
	}
	return nil
}
