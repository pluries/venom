package venom

import "testing"

func Test_RemoveNotPrintableChar(t *testing.T) {
	type args struct {
		in string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "remove U+001B escape code (not printable)",
			args: args{in: "python-mysqldb : \x1b[34mOK\x1b[0m"},
			want: "python-mysqldb :  [34mOK [0m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveNotPrintableChar(tt.args.in); got != tt.want {
				t.Errorf("RemoveNotPrintableChar() = %v, want %v", got, tt.want)
			}
		})
	}
}
