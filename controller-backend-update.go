// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/haproxytech/models"
)

type backend models.Backend

func (b *backend) updateBalance(data *StringW) error {
	//TODO Balance proper usage
	val := &models.Balance{
		Algorithm: &data.Value,
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("balance algorithm: %s", err)
	}
	b.Balance = val
	return nil
}

func (b *backend) updateCheckTimeout(data *StringW) error {
	val, err := ParseTime(data.Value)
	if err != nil {
		return fmt.Errorf("timeout check: %s", err)
	}
	b.CheckTimeout = val
	return nil
}

func (b *backend) updateForwardfor(data *StringW) error {
	if b.Mode == string(ModeTCP) {
		if data.Status != EMPTY {
			log.Printf("'option forwardfor' ignored for backend '%s' as it requires HTTP mode", b.Name)
		}
		b.Forwardfor = nil
		return nil
	}
	enabled, err := strconv.ParseBool(data.Value)
	if err != nil {
		return err
	}
	if enabled {
		b.Forwardfor = &models.Forwardfor{
			Enabled: ptrString("enabled"),
		}
	} else {
		b.Forwardfor = nil
	}
	return nil
}

func (b *backend) updateHttpchk(data *StringW) error {
	var val *models.Httpchk
	httpCheckParams := strings.Fields(strings.TrimSpace(data.Value))
	switch len(httpCheckParams) {
	case 0:
		return fmt.Errorf("httpchk option: incorrect number of params")
	case 1:
		val = &models.Httpchk{
			URI: httpCheckParams[0],
		}
	case 2:
		val = &models.Httpchk{
			Method: httpCheckParams[0],
			URI:    httpCheckParams[1],
		}
	default:
		val = &models.Httpchk{
			Method:  httpCheckParams[0],
			URI:     httpCheckParams[1],
			Version: strings.Join(httpCheckParams[2:], " "),
		}
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("httpchk option: %s", err)
	}
	b.Httpchk = val
	return nil
}

func (b *backend) updateAbortOnClose(data *StringW) error {
	if data.Value == "enabled" {
		b.Abortonclose = "enabled"
	} else {
		b.Abortonclose = ""
	}
	return nil
}
