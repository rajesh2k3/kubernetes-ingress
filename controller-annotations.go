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

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleDefaultTimeouts() bool {
	hasChanges := false
	hasChanges = c.handleDefaultTimeout("http-request", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("connect", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("client", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("queue", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("server", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("tunnel", true) || hasChanges
	hasChanges = c.handleDefaultTimeout("http-keep-alive", true) || hasChanges
	//no default values
	//timeout check is put in every backend, no need to put it here
	//hasChanges = c.handleDefaultTimeout("check", false) || hasChanges
	return hasChanges
}

func (c *HAProxyController) handleDefaultTimeout(timeout string, hasDefault bool) bool {
	config, _ := c.ActiveConfiguration()
	annTimeout, err := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if err != nil {
		if hasDefault {
			log.Println(err)
		}
		return false
	}
	if annTimeout.Status != "" {
		//log.Println(fmt.Sprintf("timeout [%s]", timeout), annTimeout.Value, annTimeout.OldValue, annTimeout.Status)
		data, err := config.Get(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout))
		if err != nil {
			if hasDefault {
				log.Println(err)
				return false
			}
			errSet := config.Set(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout), types.SimpleTimeout{
				Value: annTimeout.Value,
			})
			if errSet != nil {
				log.Println(errSet)
			}
			return true
		}
		timeout := data.(*types.SimpleTimeout)
		timeout.Value = annTimeout.Value
		return true
	}
	return false
}

// Update backend with annotations values.
func (c *HAProxyController) handleBackendAnnotations(ingress *Ingress, service *Service, backendName string, newBackend bool) (needReload bool, err error) {
	needReload = false
	model, _ := c.backendGet(backendName)
	backend := backend(model)
	backendAnnotations := make(map[string]*StringW, 4)

	backendAnnotations["abortonclose"], _ = GetValueFromAnnotations("abortonclose", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["check-http"], _ = GetValueFromAnnotations("check-http", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["forwarded-for"], _ = GetValueFromAnnotations("forwarded-for", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["load-balance"], _ = GetValueFromAnnotations("load-balance", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	backendAnnotations["timeout-check"], _ = GetValueFromAnnotations("timeout-check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)

	// The DELETED status of an annotation is handled explicitly
	// only when there is no default annotation value.
	for k, v := range backendAnnotations {
		if v == nil {
			continue
		}
		if v.Status != EMPTY || newBackend {
			switch k {
			case "abortonclose":
				if err := backend.updateAbortOnClose(v); err != nil {
					LogErr(err)
					continue
				}
				needReload = true
			case "check-http":
				if v.Status == DELETED && !newBackend {
					backend.Httpchk = nil
				} else if err := backend.updateHttpchk(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				needReload = true
			case "forwarded-for":
				if err := backend.updateForwardfor(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				needReload = true
			case "load-balance":
				if err := backend.updateBalance(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				needReload = true
			case "timeout-check":
				if v.Status == DELETED && !newBackend {
					backend.CheckTimeout = nil
				} else if err := backend.updateCheckTimeout(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				needReload = true
			}
		}
	}
	if needReload {
		if err := c.backendEdit(models.Backend(backend)); err != nil {
			return needReload, err
		}
	}
	return needReload, nil

}

// Update server with annotations values.
func (c *HAProxyController) handleServerAnnotations(ingress *Ingress, service *Service, model *models.Server) (annnotationsActive bool) {
	annnotationsActive = false
	server := server(*model)

	serverAnnotations := make(map[string]*StringW, 3)
	serverAnnotations["check"], _ = GetValueFromAnnotations("check", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	serverAnnotations["check-interval"], _ = GetValueFromAnnotations("check-interval", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	serverAnnotations["pod-maxconn"], _ = GetValueFromAnnotations("pod-maxconn", service.Annotations)

	// The DELETED status of an annotation is handled explicitly
	// only when there is no default annotation value.
	for k, v := range serverAnnotations {
		if v == nil {
			continue
		}
		if v.Status != EMPTY {
			switch k {
			case "check":
				if err := server.updateCheck(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				annnotationsActive = true
			case "check-interval":
				if v.Status == DELETED {
					server.Inter = nil
				} else if err := server.updateInter(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				annnotationsActive = true
			case "pod-maxconn":
				if v.Status == DELETED {
					server.Maxconn = nil
				} else if err := server.updateMaxconn(v); err != nil {
					LogErr(fmt.Errorf("%s annotation: %s", k, err))
					continue
				}
				annnotationsActive = true
			}
		}
	}
	return annnotationsActive
}

func (c *HAProxyController) handleSSLPassthrough(ingress *Ingress, service *Service, path *IngressPath, backendName string) {
	annSSLPassthrough, _ := GetValueFromAnnotations("ssl-passthrough", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	if annSSLPassthrough.Status != EMPTY || path.Status != EMPTY {
		enabled, err := strconv.ParseBool(annSSLPassthrough.Value)
		if err != nil {
			LogErr(err)
			return
		}
		backend, err := c.backendGet(backendName)

		if enabled {
			path.IsSSLPassthrough = true
		} else {
			path.IsSSLPassthrough = false
		}
		if err == nil {
			if path.IsSSLPassthrough {
				backend.Mode = "tcp"
			} else {
				backend.Mode = "http"
			}
			LogErr(c.backendEdit(backend))
		}
		path.Status = MODIFIED
	}
}

func (c *HAProxyController) handleRateLimitingAnnotations(ingress *Ingress, service *Service, path *IngressPath) {
	//Annotations with default values don't need error checking.
	annWhitelist, _ := GetValueFromAnnotations("whitelist", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	annWhitelistRL, _ := GetValueFromAnnotations("whitelist-with-rate-limit", service.Annotations, ingress.Annotations, c.cfg.ConfigMap.Annotations)
	allowRateLimiting := annWhitelistRL.Value != "" && annWhitelistRL.Value != "OFF"
	status := annWhitelist.Status
	if status == EMPTY {
		if annWhitelistRL.Status != EMPTY {
			data, ok := c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)]
			if ok && len(data) > 0 {
				status = MODIFIED
			}
		}
		if annWhitelistRL.Value != "" && path.Status == ADDED {
			status = MODIFIED
		}
	}
	switch status {
	case ADDED, MODIFIED:
		if annWhitelist.Value != "" {
			ID := int64(0)
			httpRequest1 := &models.HTTPRequestRule{
				ID:       &ID,
				Type:     "allow",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s } { src %s }", path.Path, strings.Replace(annWhitelist.Value, ",", " ", -1)),
			}
			httpRequest2 := &models.HTTPRequestRule{
				ID:       &ID,
				Type:     "deny",
				Cond:     "if",
				CondTest: fmt.Sprintf("{ path_beg %s }", path.Path),
			}
			if allowRateLimiting {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{
					*httpRequest1,
				}
			} else {
				c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{
					*httpRequest2, //reverse order
					*httpRequest1,
				}
			}
		} else {
			c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{}
		}
		c.cfg.HTTPRequestsStatus = MODIFIED
	case DELETED:
		c.cfg.HTTPRequests[fmt.Sprintf("WHT-%s", path.Path)] = []models.HTTPRequestRule{}
	}
}
