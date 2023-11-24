package redis

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/scaleway/scaleway-cli/v2/internal/core"
	"github.com/scaleway/scaleway-sdk-go/api/redis/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

const redisActionTimeout = 15 * time.Minute

func clusterCreateBuilder(c *core.Command) *core.Command {
	type redisEndpointSpecPrivateNetworkSpecCustom struct {
		*redis.EndpointSpecPrivateNetworkSpec
		EnableIpam bool `json:"enable-ipam"`
	}

	type redisEndpointSpecCustom struct {
		PrivateNetwork *redisEndpointSpecPrivateNetworkSpecCustom `json:"private-network"`
	}

	type redisCreateClusterRequestCustom struct {
		*redis.CreateClusterRequest
		Endpoints []*redisEndpointSpecCustom `json:"endpoints"`
	}

	c.ArgSpecs.AddBefore("endpoints.{index}.private-network.id", &core.ArgSpec{
		Name:     "endpoints.{index}.private-network.enable-ipam",
		Short:    "Will configure your Private Network endpoint with Scaleway IPAM service if true",
		Required: false,
		Default:  core.DefaultValueSetter("false"),
	})

	c.ArgsType = reflect.TypeOf(redisCreateClusterRequestCustom{})

	c.WaitFunc = func(ctx context.Context, argsI, respI interface{}) (interface{}, error) {
		api := redis.NewAPI(core.ExtractClient(ctx))
		cluster, err := api.WaitForCluster(&redis.WaitForClusterRequest{
			ClusterID:     respI.(*redis.Cluster).ID,
			Zone:          respI.(*redis.Cluster).Zone,
			Timeout:       scw.TimeDurationPtr(redisActionTimeout),
			RetryInterval: core.DefaultRetryInterval,
		})
		if err != nil {
			return nil, err
		}
		return cluster, nil
	}

	c.Run = func(ctx context.Context, argsI interface{}) (interface{}, error) {
		client := core.ExtractClient(ctx)
		api := redis.NewAPI(client)

		customRequest := argsI.(*redisCreateClusterRequestCustom)
		createClusterRequest := customRequest.CreateClusterRequest

		for _, customEndpoint := range customRequest.Endpoints {
			if customEndpoint.PrivateNetwork == nil {
				continue
			}
			ipamConfig := &redis.EndpointSpecPrivateNetworkSpecIpamConfig{}
			if !customEndpoint.PrivateNetwork.EnableIpam {
				ipamConfig = nil
			}
			createClusterRequest.Endpoints = append(createClusterRequest.Endpoints, &redis.EndpointSpec{
				PrivateNetwork: &redis.EndpointSpecPrivateNetworkSpec{
					ID:         customEndpoint.PrivateNetwork.ID,
					ServiceIPs: customEndpoint.PrivateNetwork.ServiceIPs,
					IpamConfig: ipamConfig,
				},
			})
		}

		cluster, err := api.CreateCluster(createClusterRequest)
		if err != nil {
			return nil, err
		}
		return cluster, nil
	}

	return c
}

func clusterDeleteBuilder(c *core.Command) *core.Command {
	c.WaitFunc = func(ctx context.Context, argsI, respI interface{}) (interface{}, error) {
		api := redis.NewAPI(core.ExtractClient(ctx))
		cluster, err := api.WaitForCluster(&redis.WaitForClusterRequest{
			ClusterID:     respI.(*redis.Cluster).ID,
			Zone:          respI.(*redis.Cluster).Zone,
			Timeout:       scw.TimeDurationPtr(redisActionTimeout),
			RetryInterval: core.DefaultRetryInterval,
		})
		if err != nil {
			// if we get a 404 here, it means the resource was successfully deleted
			notFoundError := &scw.ResourceNotFoundError{}
			responseError := &scw.ResponseError{}
			if errors.As(err, &responseError) && responseError.StatusCode == http.StatusNotFound || errors.As(err, &notFoundError) {
				return cluster, nil
			}
			return nil, err
		}
		return cluster, nil
	}
	return c
}

func clusterWaitCommand() *core.Command {
	return &core.Command{
		Short:     "Wait for a Redis cluster to reach a stable state",
		Long:      "Wait for a Redis cluster to reach a stable state. This is similar to using --wait flag.",
		Namespace: "redis",
		Resource:  "cluster",
		Verb:      "wait",
		ArgsType:  reflect.TypeOf(redis.WaitForClusterRequest{}),
		Run: func(ctx context.Context, argsI interface{}) (i interface{}, err error) {
			api := redis.NewAPI(core.ExtractClient(ctx))
			return api.WaitForCluster(&redis.WaitForClusterRequest{
				Zone:          argsI.(*redis.WaitForClusterRequest).Zone,
				ClusterID:     argsI.(*redis.WaitForClusterRequest).ClusterID,
				Timeout:       argsI.(*redis.WaitForClusterRequest).Timeout,
				RetryInterval: core.DefaultRetryInterval,
			})
		},
		ArgSpecs: core.ArgSpecs{
			{
				Name:       "cluster-id",
				Short:      "ID of the cluster you want to wait for",
				Required:   true,
				Positional: true,
			},
			core.ZoneArgSpec(scw.ZoneFrPar1, scw.ZoneFrPar2, scw.ZoneNlAms1, scw.ZoneNlAms2, scw.ZonePlWaw1, scw.ZonePlWaw2),
			core.WaitTimeoutArgSpec(redisActionTimeout),
		},
		Examples: []*core.Example{
			{
				Short:    "Wait for a Redis cluster to reach a stable state",
				ArgsJSON: `{"cluster-id": "11111111-1111-1111-1111-111111111111"}`,
			},
		},
	}
}