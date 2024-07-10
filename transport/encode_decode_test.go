package transport

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func Test_isIdentifierValid(t *testing.T) {
	type args struct {
		identifier string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Alphanumeric is valid",
			args{
				identifier: "TargetName123",
			},
			true,
		},
		{
			"Spaces are invalid",
			args{
				identifier: "target name",
			},
			false,
		},
		{
			"Special characters are invalid",
			args{
				identifier: "target$({}><?/",
			},
			false,
		},
		{
			"Emails are valid",
			args{
				identifier: "test@harness.io",
			},
			true,
		},
		{
			"Underscore is valid",
			args{
				identifier: "__global__cf_target",
			},
			true,
		},
		{
			"Dash is valid",
			args{
				identifier: "test-user",
			},
			true,
		},
		{
			"Single character is valid",
			args{
				identifier: "t",
			},
			true,
		},
		{
			"Empty string is invalid",
			args{
				identifier: "",
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIdentifierValid(tt.args.identifier); got != tt.want {
				t.Errorf("IsIdentifierValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNameValid(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Alphanumeric is valid",
			args{
				name: "TargetName123",
			},
			true,
		},
		{
			"Spaces are valid",
			args{
				name: "Global Target",
			},
			true,
		},
		{
			"Special characters are invalid",
			args{
				name: "target$({}><?/",
			},
			false,
		},
		{
			"Emails are valid",
			args{
				name: "test@harness.io",
			},
			true,
		},
		{
			"Underscore is valid",
			args{
				name: "__global__cf_target",
			},
			true,
		},
		{
			"Dash is valid",
			args{
				name: "test-user",
			},
			true,
		},
		{
			"Empty string is invalid",
			args{
				name: "",
			},
			false,
		},
		{
			"Unicode characters are valid",
			args{
				name: "ńoooo",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNameValid(tt.args.name); got != tt.want {
				t.Errorf("IsIdentifierValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeGetEvaluationsRequest(t *testing.T) {
	target := domain.Target{
		Target: clientgen.Target{
			Attributes: domain.ToPtr(map[string]interface{}{
				"email": "foo@gmail.com",
			}),
			Identifier: "foo",
			Name:       "bar",
		},
	}

	b, err := jsoniter.Marshal(target)
	assert.Nil(t, err)

	encodedTarget := base64.StdEncoding.EncodeToString(b)

	type args struct {
		envID   string
		target  string
		headers map[string]string
	}

	type expected struct {
		req domain.EvaluationsRequest
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I make a evaluations request and include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				headers: map[string]string{targetHeader: encodedTarget},
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsRequest{
					EnvironmentID:    "123",
					TargetIdentifier: "foo",
					Target:           &target,
				},
			},
		},
		"Given I make a evaluations request and don't include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				headers: nil,
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsRequest{
					EnvironmentID:    "123",
					TargetIdentifier: "foo",
					Target:           nil,
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for h, v := range tc.args.headers {
				req.Header.Set(h, v)
			}

			e := echo.New()
			c := e.NewContext(req, httptest.NewRecorder())
			c.SetPath(evaluationsFlagRoute)
			c.SetParamNames("environment_uuid", "target")
			c.SetParamValues(tc.args.envID, tc.args.target)

			actual, err := decodeGetEvaluationsRequest(c, log.NoOpLogger{})
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.req, actual)
		})
	}
}

func Test_decodeGetEvaluationsByFeatureRequest(t *testing.T) {
	target := domain.Target{
		Target: clientgen.Target{
			Attributes: domain.ToPtr(map[string]interface{}{
				"email": "foo@gmail.com",
			}),
			Identifier: "foo",
			Name:       "bar",
		},
	}

	b, err := jsoniter.Marshal(target)
	assert.Nil(t, err)

	encodedTarget := base64.StdEncoding.EncodeToString(b)

	type args struct {
		envID   string
		target  string
		feature string
		headers map[string]string
	}

	type expected struct {
		req domain.EvaluationsByFeatureRequest
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I make a evaluations by feature request and include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				feature: "booleanFlag",
				headers: map[string]string{targetHeader: encodedTarget},
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsByFeatureRequest{
					EnvironmentID:     "123",
					TargetIdentifier:  "foo",
					FeatureIdentifier: "booleanFlag",
					Target:            &target,
				},
			},
		},
		"Given I make a evaluations by feature request and don't include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				feature: "booleanFlag",
				headers: nil,
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsByFeatureRequest{
					EnvironmentID:     "123",
					TargetIdentifier:  "foo",
					FeatureIdentifier: "booleanFlag",
					Target:            nil,
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for h, v := range tc.args.headers {
				req.Header.Set(h, v)
			}

			e := echo.New()
			c := e.NewContext(req, httptest.NewRecorder())
			c.SetPath(evaluationsFlagRoute)
			c.SetParamNames("environment_uuid", "target", "feature")
			c.SetParamValues(tc.args.envID, tc.args.target, tc.args.feature)

			actual, err := decodeGetEvaluationsByFeatureRequest(c, log.NoOpLogger{})
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.req, actual)
		})
	}
}

func benchmarkEvalResponse() []clientgen.Evaluation {
	d := []byte(`[{"flag":"obfuscated_flag_1","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_2","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_3","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_4","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_5","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_6","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_7","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_8","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_9","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_10","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_11","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_12","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_13","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_14","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_15","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_16","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_17","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_18","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_19","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_20","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_21","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_22","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_23","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_24","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_25","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_26","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_27","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_28","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_29","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_30","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_31","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_32","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_33","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_34","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_35","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_36","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_37","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_38","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_39","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_40","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_41","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_42","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_43","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_44","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_45","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_46","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_47","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_48","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_49","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_50","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_51","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_52","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_53","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_54","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_55","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_56","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_57","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_58","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_59","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_60","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_61","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_62","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_63","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_64","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_65","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_66","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_67","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_68","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_69","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_70","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_71","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_72","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_73","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_74","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_75","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_76","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_77","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_78","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_79","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_80","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_81","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_82","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_83","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_84","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_85","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_86","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_87","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_88","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_89","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_90","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_91","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_92","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_93","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_94","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_95","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_96","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_97","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_98","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_99","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_100","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_101","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_102","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_103","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_104","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_105","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_106","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_107","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_108","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_109","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_110","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_111","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_112","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_113","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_114","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_115","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_116","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_117","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_118","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_119","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_120","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_121","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_122","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_123","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_124","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_125","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_126","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_127","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_128","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_129","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_130","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_131","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_132","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_133","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_134","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_135","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_136","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_137","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_138","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_139","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_140","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_141","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_142","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_143","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_144","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_145","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_146","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_147","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_148","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_149","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_150","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_151","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_152","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_153","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_154","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_155","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_156","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_157","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_158","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_159","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_160","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_161","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_162","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_163","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_164","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_165","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_166","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_167","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_168","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_169","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_170","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_171","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_172","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_173","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_174","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_175","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_176","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_177","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_178","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_179","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_180","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_181","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_182","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_183","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_184","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_185","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_186","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_187","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_188","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_189","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_190","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_191","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_192","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_193","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_194","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_195","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_196","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_197","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_198","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_199","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_200","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_201","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_202","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_203","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_204","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_205","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_206","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_207","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_208","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_209","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_210","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_211","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_212","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_213","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_214","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_215","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_216","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_217","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_218","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_219","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_220","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_221","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_222","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_223","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_224","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_225","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_226","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_227","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_228","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_229","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_230","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_231","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_232","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_233","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_234","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_235","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_236","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_237","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_238","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_239","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_240","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_241","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_242","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_243","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_244","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_245","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_246","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_247","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_248","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_249","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_250","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_251","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_252","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_253","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_254","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_255","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_256","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_257","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_258","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_259","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_260","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_261","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_262","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_263","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_264","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_265","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_266","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_267","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_268","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_269","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_270","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_271","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_272","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_273","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_274","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_275","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_276","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_277","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_278","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_279","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_280","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_281","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_282","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_283","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_284","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_285","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_286","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_287","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_288","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_289","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_290","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_291","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_292","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_293","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_294","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_295","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_296","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_297","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_298","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_299","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_300","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_301","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_302","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_303","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_304","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_305","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_306","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_307","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_308","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_309","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_310","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_311","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_312","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_313","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_314","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_315","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_316","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_317","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_318","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_319","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_320","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_321","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_322","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_323","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_324","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_325","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_326","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_327","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_328","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_329","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_330","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_331","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_332","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_333","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_334","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_335","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_336","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_337","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_338","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_339","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_340","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_341","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_342","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_343","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_344","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_345","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_346","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_347","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_348","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_349","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_350","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_351","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_352","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_353","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_354","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_355","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_356","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_357","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_358","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_359","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_360","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_361","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_362","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_363","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_364","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_365","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_366","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_367","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_368","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_369","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_370","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_371","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_372","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_373","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_374","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_375","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_376","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_377","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_378","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_379","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_380","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_381","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_382","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_383","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_384","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_385","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_386","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_387","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_388","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_389","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_390","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_391","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_392","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_393","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_394","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_395","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_396","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_397","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_398","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_399","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_400","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_401","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_402","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_403","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_404","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_405","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_406","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_407","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_408","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_409","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_410","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_411","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_412","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_413","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_414","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_415","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_416","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_417","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_418","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_419","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_420","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_421","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_422","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_423","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_424","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_425","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_426","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_427","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_428","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_429","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_430","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_431","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_432","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_433","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_434","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_435","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_436","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_437","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_438","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_439","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_440","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_441","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_442","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_443","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_444","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_445","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_446","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_447","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_448","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_449","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_450","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_451","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_452","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_453","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_454","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_455","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_456","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_457","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_458","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_459","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_460","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_461","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_462","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_463","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_464","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_465","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_466","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_467","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_468","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_469","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_470","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_471","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_472","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_473","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_474","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_475","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_476","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_477","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_478","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_479","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_480","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_481","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_482","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_483","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_484","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_485","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_486","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_487","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_488","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_489","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_490","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_491","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_492","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_493","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_494","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_495","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_496","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_497","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_498","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_499","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_500","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_501","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_502","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_503","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_504","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_505","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_506","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_507","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_508","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_509","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_510","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_511","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_512","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_513","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_514","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_515","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_516","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_517","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_518","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_519","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_520","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_521","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_522","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_523","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_524","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_525","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_526","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_527","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_528","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_529","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_530","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_531","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_532","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_533","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_534","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_535","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_536","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_537","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_538","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_539","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_540","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_541","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_542","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_543","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_544","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_545","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_546","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_547","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_548","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_549","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_550","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_551","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_552","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_553","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_554","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_555","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_556","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_557","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_558","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_559","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_560","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_561","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_562","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_563","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_564","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_565","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_566","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_567","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_568","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_569","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_570","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_571","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_572","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_573","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_574","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_575","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_576","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_577","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_578","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_579","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_580","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_581","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_582","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_583","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_584","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_585","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_586","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_587","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_588","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_589","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_590","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_591","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_592","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_593","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_594","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_595","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_596","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_597","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_598","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_599","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_600","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_601","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_602","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_603","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_604","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_605","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_606","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_607","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_608","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_609","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_610","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_611","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_612","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_613","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_614","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_615","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_616","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_617","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_618","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_619","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_620","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_621","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_622","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_623","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_624","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_625","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_626","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_627","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_628","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_629","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_630","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_631","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_632","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_633","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_634","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_635","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_636","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_637","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_638","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_639","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_640","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_641","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_642","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_643","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_644","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_645","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_646","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_647","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_648","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_649","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_650","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_651","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_652","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_653","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_654","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_655","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_656","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_657","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_658","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_659","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_660","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_661","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_662","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_663","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_664","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_665","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_666","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_667","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_668","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_669","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_670","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_671","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_672","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_673","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_674","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_675","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_676","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_677","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_678","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_679","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_680","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_681","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_682","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_683","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_684","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_685","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_686","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_687","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_688","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_689","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_690","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_691","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_692","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_693","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_694","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_695","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_696","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_697","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_698","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_699","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_700","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_701","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_702","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_703","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_704","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_705","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_706","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_707","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_708","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_709","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_710","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_711","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_712","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_713","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_714","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_715","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_716","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_717","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_718","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_719","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_720","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_721","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_722","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_723","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_724","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_725","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_726","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_727","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_728","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_729","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_730","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_731","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_732","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_733","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_734","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_735","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_736","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_737","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_738","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_739","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_740","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_741","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_742","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_743","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_744","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_745","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_746","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_747","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_748","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_749","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_750","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_751","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_752","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_753","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_754","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_755","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_756","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_757","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_758","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_759","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_760","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_761","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_762","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_763","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_764","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_765","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_766","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_767","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_768","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_769","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_770","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_771","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_772","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_773","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_774","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_775","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_776","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_777","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_778","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_779","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_780","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_781","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_782","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_783","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_784","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_785","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_786","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_787","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_788","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_789","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_790","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_791","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_792","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_793","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_794","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_795","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_796","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_797","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_798","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_799","identifier":"false","kind":"boolean","value":"false"},{"flag":"obfuscated_flag_800","identifier":"false","kind":"boolean","value":"false"}]
`)

	var resp []clientgen.Evaluation
	if err := jsoniter.Unmarshal(d, &resp); err != nil {
		panic("ahh")
	}

	return resp

}

func Benchmark_EncodeResponse(b *testing.B) {
	response := benchmarkEvalResponse()

	newline := []byte{'\n'}

	benchmarks := []struct {
		name       string
		encodeFunc func(interface{}, http.ResponseWriter) error
	}{
		{"jsoniter.Marshal", func(i interface{}, w http.ResponseWriter) error {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			b, err := jsoniter.Marshal(response)
			if err != nil {
				return err
			}

			_, err = w.Write(b)
			if err != nil {
				return err
			}

			_, err = w.Write(newline)
			if err != nil {
				return err
			}
			return nil
		}},
		{"json.Marshal", func(i interface{}, w http.ResponseWriter) error {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			b, err := json.Marshal(response)
			if err != nil {
				return err
			}

			_, err = w.Write(b)
			if err != nil {
				return err
			}

			_, err = w.Write(newline)
			if err != nil {
				return err
			}
			return nil
		}},
		{"json.NewEncoder", func(i interface{}, w http.ResponseWriter) error {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			encoder := json.NewEncoder(w)
			if err := encoder.Encode(response); err != nil {
				return err
			}

			return nil
		}},
		{
			"jsoniter.NewEncoder", func(i interface{}, w http.ResponseWriter) error {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")

				encoder := jsoniter.NewEncoder(w)
				if err := encoder.Encode(response); err != nil {
					return err
				}

				return nil
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				rec := httptest.NewRecorder()
				if err := bm.encodeFunc(response, rec); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
