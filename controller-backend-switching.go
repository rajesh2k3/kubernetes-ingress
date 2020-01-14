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
	"sort"

	"github.com/haproxytech/models"
)

type UseBackendRules map[string]UseBackendRule

type UseBackendRule struct {
	Host      string
	Path      string
	Backend   string
	Namespace string
}

func (c *HAProxyController) addUseBackendRule(key string, rule UseBackendRule, frontends ...string) {
	for _, frontendName := range frontends {
		c.cfg.BackendSwitchingRules[frontendName][key] = rule
		c.cfg.BackendSwitchingStatus[frontendName] = struct{}{}
	}
}

func (c *HAProxyController) deleteUseBackendRule(key string, frontends ...string) {
	for _, frontendName := range frontends {
		delete(c.cfg.BackendSwitchingRules[frontendName], key)
		c.cfg.BackendSwitchingStatus[frontendName] = struct{}{}
	}
}

//  Recreate use_backend rules
func (c *HAProxyController) refreshBackendSwitching() (needsReload bool) {
	if len(c.cfg.BackendSwitchingStatus) == 0 {
		return false
	}
	frontends, err := c.frontendsGet()
	if err != nil {
		PanicErr(err)
		return false
	}
	// Active backend will hold backends in use
	activeBackends := map[string]struct{}{"RateLimit": struct{}{}}
	for _, frontend := range frontends {
		activeBackends[frontend.DefaultBackend] = struct{}{}
		useBackendRules, ok := c.cfg.BackendSwitchingRules[frontend.Name]
		if !ok {
			continue
		}
		sortedKeys := []string{}
		for key, rule := range useBackendRules {
			activeBackends[rule.Backend] = struct{}{}
			sortedKeys = append(sortedKeys, key)
		}
		if _, ok := c.cfg.BackendSwitchingStatus[frontend.Name]; !ok {
			// No need to refresh rules if the use_backend rules
			// of the frontend were not updated
			continue
		}
		// host/path are part of use_backend keys, so sorting keys will
		// result in sorted use_backend rules for better readability
		sort.Sort(sort.Reverse(sort.StringSlice(sortedKeys)))
		c.backendSwitchingRuleDeleteAll(frontend.Name)
		for _, key := range sortedKeys {
			rule := useBackendRules[key]
			var condTest string
			switch frontend.Mode {
			case "http":
				if rule.Host != "" {
					condTest = fmt.Sprintf("{ req.hdr(host) -i %s } ", rule.Host)
				}
				if rule.Path != "" {
					condTest = fmt.Sprintf("%s{ path_beg %s }", condTest, rule.Path)
				}
				if condTest == "" {
					log.Println("Both Host and Path are empty for frontend %s with backend %s, SKIP", frontend, rule.Backend)
					continue
				}
			case "tcp":
				if rule.Host == "" {
					log.Println(fmt.Sprintf("Empty SNI for backend %s, SKIP", rule.Backend))
					continue
				}
				condTest = fmt.Sprintf("{ req_ssl_sni -i %s } ", rule.Host)
			}
			err := c.backendSwitchingRuleCreate(frontend.Name, models.BackendSwitchingRule{
				Cond:     "if",
				CondTest: condTest,
				Name:     rule.Backend,
				ID:       ptrInt64(0),
			})
			PanicErr(err)
		}
		needsReload = true
		delete(c.cfg.BackendSwitchingStatus, frontend.Name)
	}
	needsReload = needsReload || c.clearBackends(activeBackends)
	return needsReload
}

// Remove unused backends
func (c *HAProxyController) clearBackends(activeBackends map[string]struct{}) (needsReload bool) {
	allBackends, err := c.backendsGet()
	if err != nil {
		PanicErr(err)
		return false
	}
	for _, backend := range allBackends {
		if _, ok := activeBackends[backend.Name]; !ok {
			if err := c.backendDelete(backend.Name); err != nil {
				PanicErr(err)
			}
			needsReload = true
		}
	}
	return needsReload
}
