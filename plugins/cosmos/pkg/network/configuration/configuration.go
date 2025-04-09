package configuration

import (
	"context"

	crossplanev1beta1 "github.com/web-seven/overlock-api/go/node/overlock/crossplane/v1beta1"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func CreateConfiguration(ctx context.Context, logger *zap.SugaredLogger, msg crossplanev1beta1.MsgCreateConfiguration, config *rest.Config, dc *dynamic.DynamicClient) {
	packageEncrypted := msg.Spec.GetPackage()
	aesKeyEncrypted := msg.Spec.GetAesKey()

	packageURL, err := DecryptFromKeyring(packageEncrypted, logger)
	if err != nil {
		logger.Errorf("Failed to decode package: %v", err)
		return
	}

	aesKey, err := DecryptFromKeyring(aesKeyEncrypted, logger)
	if err != nil {
		logger.Errorf("Failed to decode AES key: %v", err)
		return
	}
	yamlData, error := fetchPackage(ctx, packageURL, aesKey)
	if error != nil {
		logger.Errorf("Failed to fetch package: %v", error)
		return
	}

	tarBuf, err := PackageYamlToImageTarball(yamlData, packageURL)
	if err != nil {
		logger.Errorf("Failed to create image tarball: %v", err)
		return
	}

	err = LoadConfigFromTar(ctx, msg.Metadata.Name, config, logger, tarBuf)
	if err != nil {
		logger.Errorf("Failed to load configuration from memory: %v", err)
		return
	}

}
