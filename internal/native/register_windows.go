package native

import "golang.org/x/sys/windows/registry"

func register(path string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Google\Chrome\NativeMessagingHosts\`+HostName, registry.SET_VALUE|registry.WOW64_32KEY)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("", path)
}
