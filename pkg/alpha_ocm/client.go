package alphaocm

type OcmClient interface {
	CreateWifConfig(wifConfigInput interface{}) error
}
