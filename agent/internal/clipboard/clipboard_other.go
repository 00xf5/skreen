//go:build !windows

package clipboard

func Get() (string, error) {
	return "", nil
}

func Set(text string) error {
	return nil
}
