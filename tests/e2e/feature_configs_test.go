package e2e

//var (
//	expectedStringVariation = "blue"
//	expectedBoolVariation   = "true"
//
//	// subset of expected flag data
//	expectedFlags = map[string]client.FeatureConfig{"string-flag1": {
//		DefaultServe: client.Serve{nil, &expectedStringVariation},
//		Environment:  "default",
//		Feature:      "string-flag1",
//		Kind:         "string",
//		OffVariation: "red",
//		State:        "off",
//	}, "bool-flag1": {
//		DefaultServe: client.Serve{nil, &expectedBoolVariation},
//		Environment:  "default",
//		Feature:      "bool-flag1",
//		Kind:         "boolean",
//		OffVariation: "true",
//		State:        "off",
//	}}
//)
//
//// Test GET /client/env/{environmentUUID}/feature-configs
//func TestGetFeatureConfig(t *testing.T) {
//	token, claims, err := testhelpers.AuthenticateSDKClient(GetServerAPIKey(), GetStreamURL(), nil)
//	if err != nil {
//		t.Error(err)
//	}
//
//	emptyProjToken, emptyProjClaims, err := testhelpers.AuthenticateSDKClient(GetEmptyProjectServerAPIKey(), GetStreamURL(), nil)
//	if err != nil {
//		if GetEmptyProjectServerAPIKey() != "" {
//			t.Error(err)
//		}
//	}
//
//	type args struct {
//		Token string
//		Env   string
//	}
//	tests := map[string]struct {
//		args    args
//		want    featureConfigsResponse
//		wantErr bool
//	}{
//		"Test GetFeatureConfigs succeeds for valid request": {
//			args{Token: token, Env: claims.Environment},
//			featureConfigsResponse{
//				Flags:      expectedFlags,
//				StatusCode: 200,
//			},
//			false,
//		},
//		// TODO - we don't return the same client.Error object as ff-server, we should add middleware to handle that
//		// functionally it doesn't make any difference because the error code is valid but it'd help users debugging in the network tab
//		"Test GetFeatureConfigs fails when JWT is invalid": {
//			args{Token: "iAmFakeDontLetMeIn", Env: claims.Environment},
//			featureConfigsResponse{
//				StatusCode: 401,
//				//Error:      &client.Error{Code: "401", Message: "invalid or expired jwt"},
//			},
//			false,
//		},
//		"Test GetFeatureConfigs succeeds for valid request for empty project": {
//			args{Token: emptyProjToken, Env: emptyProjClaims.Environment},
//			featureConfigsResponse{
//				Flags:      map[string]client.FeatureConfig{},
//				StatusCode: 200,
//			},
//			false,
//		},
//		// TODO - this comes back with empty values right now, we should match ff-server behaviour
//		//"Test GetFeatureConfigs fails when environment UUID is invalid": {
//		//	args{Token: token, Env: "32432432dasfgdafdsf"},
//		//	featureConfigsResponse{
//		//		StatusCode: 403,
//		//		Error:      &client.Error{Message: fmt.Sprintf("Environment ID %s mismatch with requested 32432432dasfgdafdsf", claims.Environment)},
//		//	},
//		//	false,
//		//},
//	}
//	for name, tt := range tests {
//		t.Run(name, func(t *testing.T) {
//			if tt.args.Token == "" {
//				t.Skipf("token not provided for test %s, skipping", t.Name())
//			}
//
//			got, err := getAllFeatureConfigs(tt.args.Env, tt.args.Token)
//
//			// Assert the client did not error
//			if (err != nil) != tt.wantErr {
//				assert.Errorf(t, err, "GetFeatureConfigs() error = %v, wantErr %v", err, tt.wantErr)
//			}
//
//			// Assert the response
//			assert.NotNil(t, got)
//			assert.Equal(t, tt.want.StatusCode, got.StatusCode(), "expected http status code %d but got %d", tt.want.StatusCode, got.StatusCode())
//			if got.StatusCode() == 200 {
//				assert.Nil(t, tt.want.Error)
//				assert.NotNil(t, got.JSON200)
//				assert.Equal(t, len(tt.want.Flags), len(*got.JSON200))
//
//				// check data matches the flags we expect
//				for _, flag := range *got.JSON200 {
//					expectedFlag, ok := expectedFlags[flag.Feature]
//					assert.True(t, ok)
//					assert.Equal(t, expectedFlag.Kind, flag.Kind)
//					assert.Equal(t, expectedFlag.State, flag.State)
//
//					assert.Equal(t, expectedFlag.Environment, flag.Environment)
//					assert.Equal(t, expectedFlag.DefaultServe, flag.DefaultServe)
//					assert.Equal(t, expectedFlag.OffVariation, flag.OffVariation)
//				}
//			} //else {
//			//	resp := client.Error{}
//			//	json.Unmarshal(got.Body, &resp)
//			//	assert.Equal(t, tt.want.Error, &resp)
//			//}
//		})
//	}
//}
//
///**
//Helper functions and structs to support the tests above
//*/
//
//// featureConfigsResponse captures the fields we could get back in a auth request
//type featureConfigsResponse struct {
//	StatusCode int
//	Flags      map[string]client.FeatureConfig
//	*client.Error
//}
//
//func getAllFeatureConfigs(envID string, token string) (*client.GetFeatureConfigResponse, error) {
//	c := testhelpers.DefaultEvaluationClient(GetStreamURL())
//	resp, err := c.GetFeatureConfig(context.Background(), envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
//		req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
//		return nil
//	})
//	if err != nil {
//		return nil, err
//	}
//	return client.ParseGetFeatureConfigResponse(resp)
//}
