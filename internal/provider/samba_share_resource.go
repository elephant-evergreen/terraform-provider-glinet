package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewSambaShareResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "samba_share",
		Description: "Manage one Samba share section read by the stock `/etc/init.d/samba4`. Values such as `read_only` are passed to smb.conf unchanged, so use `yes` or `no`.",
		Attributes: map[string]schema.Attribute{
			"section":             schema.StringAttribute{Required: true, MarkdownDescription: "UCI section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"share_name":          schema.StringAttribute{Required: true, MarkdownDescription: "Share name shown to clients."},
			"path":                schema.StringAttribute{Required: true, MarkdownDescription: "Shared directory."},
			"users":               schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Space-separated valid users."},
			"read_only":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "`yes` or `no`."},
			"writeable":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "`yes` or `no`."},
			"browseable":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "`yes` or `no`."},
			"guest_ok":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "`yes` or `no`."},
			"guest_only":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "`yes` or `no`."},
			"inherit_owner":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "`yes` or `no`."},
			"create_mask":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Create mask, such as `0666`."},
			"dir_mask":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Directory mask, such as `0777`."},
			"write_list":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Space-separated users with write access."},
			"read_list":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Space-separated users with read-only access."},
			"vfs_objects":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Space-separated VFS modules."},
			"force_root":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Force file access as root."},
			"timemachine":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Advertise the share as a Time Machine target."},
			"timemachine_maxsize": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Time Machine quota in gigabytes."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			section := stringOf(values, "section")
			if section == "" {
				return router.Section{}, fmt.Errorf("section is required")
			}
			options := map[string]string{"name": stringOf(values, "share_name"), "path": stringOf(values, "path")}
			for _, field := range []struct{ attrName, option string }{
				{"users", "users"}, {"read_only", "read_only"}, {"writeable", "writeable"},
				{"browseable", "browseable"}, {"guest_ok", "guest_ok"}, {"guest_only", "guest_only"},
				{"inherit_owner", "inherit_owner"}, {"create_mask", "create_mask"}, {"dir_mask", "dir_mask"},
				{"write_list", "write_list"}, {"read_list", "read_list"}, {"vfs_objects", "vfs_objects"},
				{"timemachine_maxsize", "timemachine_maxsize"},
			} {
				options[field.option] = stringOf(values, field.attrName)
			}
			options["force_root"] = boolString(boolOf(values, "force_root", false))
			options["timemachine"] = boolString(boolOf(values, "timemachine", false))
			return router.Section{Package: "samba4", Type: "sambashare", Name: section, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"section": stringValue(section.Name), "share_name": stringValue(section.Options["name"]),
				"path": stringValue(section.Options["path"]), "users": stringValue(section.Options["users"]),
				"read_only": stringValue(section.Options["read_only"]), "writeable": stringValue(section.Options["writeable"]),
				"browseable": stringValue(section.Options["browseable"]), "guest_ok": stringValue(section.Options["guest_ok"]),
				"guest_only": stringValue(section.Options["guest_only"]), "inherit_owner": stringValue(section.Options["inherit_owner"]),
				"create_mask": stringValue(section.Options["create_mask"]), "dir_mask": stringValue(section.Options["dir_mask"]),
				"write_list": stringValue(section.Options["write_list"]), "read_list": stringValue(section.Options["read_list"]),
				"vfs_objects": stringValue(section.Options["vfs_objects"]), "force_root": boolValue(section.Options["force_root"] == "1"),
				"timemachine":         boolValue(section.Options["timemachine"] == "1"),
				"timemachine_maxsize": stringValue(section.Options["timemachine_maxsize"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a sambashare section name")
			}
			return map[string]attr.Value{"section": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "section"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "share_name") == "" || stringOf(values, "path") == "" {
				return fmt.Errorf("share_name and path are required")
			}
			return nil
		},
	})
}
