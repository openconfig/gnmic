package app

import (
	"errors"
	"fmt"
	"os"
)

func (a *App) logError(err error) {
	if err == nil {
		return
	}
	a.Logger.Print(err)
	if !a.Config.Log {
		fmt.Fprintln(os.Stderr, err)
	}
	if a.errCh == nil {
		return
	}
	a.errCh <- err
}

func (a *App) checkErrors() error {
	if a.errCh == nil {
		return nil
	}
	close(a.errCh)
	errs := make([]error, 0)
	for err := range a.errCh {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	if a.Config.Log {
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	return errors.New("one or more requests failed")
}
