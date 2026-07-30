package env

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// nolint:paralleltest
func Test_getEnvBool(t *testing.T) {
	type args struct {
		name       string
		defaultVal bool
	}

	type envArgs struct {
		name  string
		value string
	}

	tests := []struct {
		name    string
		args    args
		envArgs envArgs
		want    bool
	}{
		{
			name: "Параметр задан в env",
			args: args{
				name:       "param_name",
				defaultVal: true,
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "true",
			},
			want: true,
		},
		{
			name: "Параметр не задан в env, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: true,
			},
			envArgs: envArgs{
				name:  "unknown_param_name",
				value: "false",
			},
			want: true,
		},
		{
			name: "Параметр задан, но не bool, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: true,
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "123456",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envArgs.name, tt.envArgs.value)
			require.Equal(t, tt.want, GetBool(tt.args.name, tt.args.defaultVal))
		})
	}
}

// nolint:paralleltest
func Test_getEnvDuration(t *testing.T) {
	type args struct {
		name       string
		defaultVal time.Duration
	}

	type envArgs struct {
		name  string
		value string
	}

	tests := []struct {
		name    string
		args    args
		envArgs envArgs
		want    time.Duration
	}{
		{
			name: "Параметр задан в env",
			args: args{
				name:       "param_name",
				defaultVal: time.Duration(5),
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "15ns",
			},
			want: time.Duration(15),
		},
		{
			name: "Параметр не задан в env, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: time.Duration(5),
			},
			envArgs: envArgs{
				name:  "unknown_param_name",
				value: "15ns",
			},
			want: time.Duration(5),
		},
		{
			name: "Параметр задан, но не число, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: time.Duration(5),
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "bla-bla-bla",
			},
			want: time.Duration(5),
		},
	}
	for _, tt := range tests {
		t.Setenv(tt.envArgs.name, tt.envArgs.value)
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GetDuration(tt.args.name, tt.args.defaultVal))
		})
		_ = os.Unsetenv(tt.envArgs.name)
	}
}

// nolint:paralleltest
func Test_getEnvInt(t *testing.T) {
	type args struct {
		name       string
		defaultVal int
	}

	type envArgs struct {
		name  string
		value string
	}

	tests := []struct {
		name    string
		args    args
		envArgs envArgs
		want    int
	}{
		{
			name: "Параметр задан в env",
			args: args{
				name:       "param_name",
				defaultVal: 11,
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "33",
			},
			want: 33,
		},
		{
			name: "Параметр не задан в env, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: 11,
			},
			envArgs: envArgs{
				name:  "unknown_param_name",
				value: "33",
			},
			want: 11,
		},
		{
			name: "Параметр задан, но не int, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: 11,
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "bla-bla",
			},
			want: 11,
		},
	}
	for _, tt := range tests {
		t.Setenv(tt.envArgs.name, tt.envArgs.value)
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GetInt(tt.args.name, tt.args.defaultVal))
		})
		_ = os.Unsetenv(tt.envArgs.name)
	}
}

// nolint:paralleltest
func Test_getEnvString(t *testing.T) {
	type args struct {
		name       string
		defaultVal string
	}

	type envArgs struct {
		name  string
		value string
	}

	tests := []struct {
		name    string
		args    args
		envArgs envArgs
		want    string
	}{
		{
			name: "Параметр задан в env",
			args: args{
				name:       "param_name",
				defaultVal: "default",
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "value from env",
			},
			want: "value from env",
		},
		{
			name: "Параметр не задан в env, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: "default",
			},
			envArgs: envArgs{
				name:  "unknown_param_name",
				value: "value from env",
			},
			want: "default",
		},
		{
			name: "Параметр задан, но пустой string, берем default значение",
			args: args{
				name:       "param_name",
				defaultVal: "default",
			},
			envArgs: envArgs{
				name:  "param_name",
				value: "",
			},
			want: "default",
		},
	}
	for _, tt := range tests {
		t.Setenv(tt.envArgs.name, tt.envArgs.value)
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, GetString(tt.args.name, tt.args.defaultVal))
		})
		_ = os.Unsetenv(tt.envArgs.name)
	}
}
