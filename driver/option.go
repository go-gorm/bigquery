package driver

type driverOption struct {
	credentialFile string
}

type applyOption func(opt *driverOption)

func WithCredentialFile(credentialFile string) applyOption {
	return func(opt *driverOption) {
		opt.credentialFile = credentialFile
	}
}
