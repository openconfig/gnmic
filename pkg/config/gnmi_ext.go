package config

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/utils"
	pkgUtils "github.com/openconfig/gnmic/pkg/utils"
)

func createAdditionalRequestExtensions(
	extensions string,
	protoDir,
	protoFiles []string,
	extensionDecodeMap utils.RegisteredExtensions,
) ([]*gnmi_ext.Extension, error) {

	var exts []*gnmi_ext.Extension

	if len(protoFiles) == 0 {
		return exts, nil
	}

	descSource, err := grpcurl.DescriptorSourceFromProtoFiles(protoDir, protoFiles...)

	if err != nil {
		return nil, err
	}

	var extensionsMap map[string]any

	if err := json.Unmarshal([]byte(extensions), &extensionsMap); err != nil {
		return nil, fmt.Errorf("extensions JSON decoding error: %w", err)
	}

	for idMsg, extMsg := range extensionsMap {
		id, err := strconv.ParseInt(idMsg, 10, 32)

		if err != nil {
			return nil, err
		}

		msg, exists := extensionDecodeMap[int32(id)]

		if !exists {
			return nil, fmt.Errorf("custom extension for the request was not found in the provided registered extensions")
		}

		desc, err := descSource.FindSymbol(msg)

		if err != nil {
			return nil, err
		}

		pm := dynamic.NewMessage(desc.GetFile().FindMessage(msg))

		msgBytes, err := json.Marshal(extMsg)

		if err != nil {
			return nil, err
		}

		if err = pm.UnmarshalJSON(msgBytes); err != nil {
			return nil, err
		}

		extBytes, err := pm.Marshal()

		if err != nil {
			return nil, err
		}

		ext := gnmi_ext.Extension_RegisteredExt{
			RegisteredExt: &gnmi_ext.RegisteredExtension{
				Id:  gnmi_ext.ExtensionID(id),
				Msg: extBytes,
			},
		}

		exts = append(exts, &gnmi_ext.Extension{Ext: &ext})
	}

	return exts, nil
}

func (c *Config) parseAdditionalRequestExtensions() ([]api.GNMIOption, error) {
	gnmiOpts := []api.GNMIOption{}

	if c.GlobalFlags.RequestExtensions == "" {
		return gnmiOpts, nil
	}

	registeredExtensions, err := pkgUtils.ParseRegisteredExtensions(c.GlobalFlags.RegisteredExtensions)

	if err != nil {
		return nil, err
	}

	exts, err := createAdditionalRequestExtensions(
		c.GlobalFlags.RequestExtensions,
		c.GlobalFlags.ProtoDir,
		c.GlobalFlags.ProtoFile,
		registeredExtensions,
	)

	if err != nil {
		return nil, err
	}

	for _, ext := range exts {
		gnmiOpts = append(gnmiOpts, api.Extension(ext))
	}

	return gnmiOpts, nil
}
