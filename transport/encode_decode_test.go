package transport

import "testing"

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
