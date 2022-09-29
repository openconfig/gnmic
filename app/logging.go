// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

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
