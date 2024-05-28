package mock

// type WifClient interface {
// 	CreateWifConfig(input models.WifConfigInput) (models.WifConfigOutput, error)
// 	GetWifConfig(wifHref string) (models.WifConfigOutput, error)
// }

// type wifClient struct {
// 	data string
// }

// func NewWifClient(file string) WifClient {
// 	return &wifClient{
// 		data: "http://localhost:1973",
// 	}
// }

// func (c *wifClient) CreateWifConfig(input models.WifConfigInput) (out models.WifConfigOutput, err error) {
// 	inputJson, err := json.Marshal(input)
// 	if err != nil {
// 		return
// 	}
// 	// Save the wifconfig to the data filepath as json

// 	output := &models.WifConfigOutput{
// 		Metadata: &models.WifConfigMetadata{
// 			DisplayName:  "",
// 			Id:           "",
// 			Organization: &models.WifConfigMetadataOrganization{},
// 		},
// 		Spec: &models.WifConfigInput{
// 			DisplayName: "",
// 			ProjectId:   "",
// 		},
// 		Status: &models.WifConfigStatus{
// 			ServiceAccounts:          []models.ServiceAccount{},
// 			State:                    "",
// 			Summary:                  "",
// 			TimeData:                 models.WifTimeData{},
// 			WorkloadIdentityPoolData: models.WifWorkloadIdentityPoolData{},
// 		},
// 	}

// 	url := fmt.Sprintf("%s/wif-configs", c.data)
// 	resp, err := http.Post(url, "application/json", bytes.NewBuffer(inputJson))
// 	if err != nil {
// 		return
// 	}
// 	defer resp.Body.Close()
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return
// 	}
// 	json.Unmarshal(body, &out)
// 	return
// }

// func (c *wifClient) GetWifConfig(wifHref string) (out models.WifConfigOutput, err error) {
// 	url := fmt.Sprintf("%s%s", c.urlRoot, wifHref)
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return
// 	}
// 	defer resp.Body.Close()
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return
// 	}
// 	json.Unmarshal(body, &out)
// 	return
// }
