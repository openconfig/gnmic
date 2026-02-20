package config

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/openconfig/gnmic/pkg/utils"
	rootUtils "github.com/openconfig/gnmic/pkg/utils"
)

type SetRegisteredExtensions map[string]any

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

	var extensionsMap SetRegisteredExtensions

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

func (c *Config) parseAdditionalRequestExtensions() ([]*gnmi_ext.Extension, error) {
	if c.GlobalFlags.AddRequestExtensions == "" {
		return []*gnmi_ext.Extension{}, nil
	}

	registeredExtensions, err := rootUtils.ParseRegisteredExtensions(c.GlobalFlags.RegisteredExtensions)

	if err != nil {
		c.logger.Printf("error parsing registered extensions: %s", err)

		return nil, err
	}

	exts, err := createAdditionalRequestExtensions(
		c.GlobalFlags.AddRequestExtensions,
		c.GlobalFlags.ProtoDir,
		c.GlobalFlags.ProtoFile,
		registeredExtensions,
	)

	if err != nil {
		c.logger.Printf("error parsing registered extensions for set-request: %s", err)

		return nil, err
	}

	return exts, nil
}
