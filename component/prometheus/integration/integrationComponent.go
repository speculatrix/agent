package integration

import (
	"context"
	"net/http"
	"path"
	"sync"

	"github.com/go-kit/log/level"
	"github.com/grafana/agent/component"
	"github.com/grafana/agent/component/discovery"
	"github.com/grafana/agent/pkg/integrations"
	"github.com/prometheus/common/model"
)

// Creator is a function provided by an implementation to create a concrete integration instance.
type Creator func(component.Options, component.Arguments) (integrations.Integration, error)

// Exports are simply a list of targets for a scraper to consume.
type Exports struct {
	Targets []discovery.Target `river:"targets,attr"`
}

type Component struct {
	opts component.Options

	mut sync.Mutex

	reload chan struct{}

	creator Creator

	integration    integrations.Integration
	metricsHandler http.Handler
}

func New(creator Creator) func(component.Options, component.Arguments) (component.Component, error) {
	return func(opts component.Options, args component.Arguments) (component.Component, error) {
		c := &Component{
			opts:    opts,
			reload:  make(chan struct{}, 1),
			creator: creator,
		}
		// Call to Update() to set the output once at the start.
		if err := c.Update(args); err != nil {
			return nil, err
		}
		targets := []discovery.Target{{
			model.AddressLabel:     opts.HTTPListenAddr,
			model.SchemeLabel:      "http",
			model.MetricsPathLabel: path.Join(opts.HTTPPath, "metrics"),
			"name":                 "node_exporter",
		}}
		opts.OnStateChange(Exports{
			Targets: targets,
		})
		return c, nil
	}
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	var cancel context.CancelFunc
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reload:
			// cancel any previously running integration
			if cancel != nil {
				cancel()
			}
			// create new context so we can cancel it if we get any future updates
			// since it is derived from the main run context, it only needs to be
			// canceled directly if we receive new updates
			newCtx, cancelFunc := context.WithCancel(ctx)
			cancel = cancelFunc

			// finally create and run new integration
			c.mut.Lock()
			integration := c.integration
			c.metricsHandler = c.getHttpHandler(integration)
			c.mut.Unlock()
			go integration.Run(newCtx)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	integration, err := c.creator(c.opts, args)
	if err != nil {
		return err
	}
	c.mut.Lock()
	c.integration = integration
	c.mut.Unlock()
	c.reload <- struct{}{}
	return err
}

// get the http handler once and save it, so we don't create uneccesary garbage
func (c *Component) getHttpHandler(integration integrations.Integration) http.Handler {
	h, err := integration.MetricsHandler()
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "failed to creating metrics handler", "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
	}
	return h
}

// Handler serves node_exporter metrics endpoint.
func (c *Component) Handler() http.Handler {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.metricsHandler
}
