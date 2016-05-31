package api

import (
	"strings"

	"github.com/gogap-micro/post-api/api/helper"
	"github.com/micro/go-micro/registry"
)

func (p *PostAPI) watch(watcher registry.Watcher) error {
	defer watcher.Stop()

	// manage this loop
	go func() {
		// wait for exit
		select {
		case <-p.stopedChan:
		}

		// stop the watcher
		watcher.Stop()
	}()

	for {
		res, err := watcher.Next()
		if err != nil {
			return err
		}
		p.updateAPIService(res)
	}
}

func (p *PostAPI) getService(api, version string) (srv microService, exist bool) {
	var srvs map[string]microService
	if srvs, exist = p.apiService[api]; exist {
		if srv, exist = srvs[version]; exist {
			return
		}
	}
	return
}

func (p *PostAPI) updateAPIService(res *registry.Result) {
	if res == nil || res.Service == nil {
		return
	}

	p.reglocker.Lock()
	defer p.reglocker.Unlock()

	switch res.Action {
	case "create", "update":
		p.createOrUpdateMicroService(res.Service)
	case "delete":
		if len(res.Service.Nodes) == 0 {
			p.removeMicroService(res.Service.Name)
		} else {
			p.removeMicroServiceOnServiceChange(res.Service)
		}
	}
}

func (p *PostAPI) removeMicroService(serviceName string) {
	for api, srvs := range p.apiService {
		for _, srv := range srvs {
			if srv.Service == serviceName {
				delete(p.apiService, api)
				break
			}
		}
	}
}

func (p *PostAPI) removeMicroServiceOnServiceChange(service *registry.Service) {
	for _, endpoint := range service.Endpoints {
		if apiMeta, exist := endpoint.Metadata[helper.APIMetadataKey]; exist {

			apis := strings.Split(apiMeta, ",")
			version := endpoint.Metadata[helper.APIVerMetadataKey]

			for _, api := range apis {
				if srvs, exist := p.apiService[api]; exist {

					if _, exist := srvs[version]; exist {
						delete(srvs, version)
					}

					if len(srvs) == 0 {
						delete(p.apiService, api)
					}
				}
			}
		}
	}
}

func (p *PostAPI) createOrUpdateMicroService(service *registry.Service) (err error) {
	for _, endpoint := range service.Endpoints {
		if apiMeta, exist := endpoint.Metadata[helper.APIMetadataKey]; exist {
			apis := strings.Split(apiMeta, ",")
			version := endpoint.Metadata[helper.APIVerMetadataKey]

			for _, api := range apis {
				if srvs, exist := p.apiService[api]; !exist {
					p.apiService[api] = map[string]microService{
						version: microService{Service: service.Name, Method: endpoint.Name},
					}

				} else if _, exist := srvs[version]; !exist {
					srvs[version] = microService{Service: service.Name, Method: endpoint.Name}
				}
			}
		}
	}

	return
}
