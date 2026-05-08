// Copyright (C) 2023-2026  Red Hat, Inc.
//
// This copyrighted material is made available to anyone wishing to use,
// modify, copy, or redistribute it subject to the terms and conditions of
// the GNU General Public License v.2, or (at your option) any later version.
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY expressed or implied, including the implied warranties of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
// Public License for more details.  You should have received a copy of the
// GNU General Public License along with this program; if not, write to the
// Free Software Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
// 02110-1301, USA.  Any Red Hat trademarks that are incorporated in the
// source code or documentation are not subject to the GNU General Public
// License and may only be used or replicated with the express permission of
// Red Hat, Inc.
//
// Red Hat Author(s): Todd Gill <tgill@redhat.com>

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/springfield-csi/driver/pkg/controller"
	"github.com/springfield-csi/driver/pkg/driver"
	"github.com/springfield-csi/driver/pkg/identity"
	"github.com/springfield-csi/driver/pkg/node"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

const (
	defaultEndpoint = "unix:///csi/csi.sock"
)

var (
	endpoint = flag.String("endpoint", defaultEndpoint, "CSI endpoint")
	nodeID   = flag.String("nodeid", "", "Node ID")
	nodeOnly = flag.Bool("nodeonly", false, "Run only node service (skip controller)")
	version  = flag.Bool("version", false, "Print version information")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *version {
		fmt.Printf("Springfield CSI Driver\n")
		fmt.Printf("  Name:    %s\n", driver.DriverName)
		fmt.Printf("  Version: %s\n", driver.DriverVersion)
		os.Exit(0)
	}

	if *nodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			klog.Fatalf("Failed to get hostname: %v", err)
		}
		*nodeID = hostname
	}

	klog.Infof("Starting Springfield CSI Driver")
	klog.Infof("  Driver:  %s", driver.DriverName)
	klog.Infof("  Version: %s", driver.DriverVersion)
	klog.Infof("  NodeID:  %s", *nodeID)
	klog.Infof("  Mode:    %s", getMode(*nodeOnly))

	if err := run(); err != nil {
		klog.Fatalf("Failed to run driver: %v", err)
	}
}

func getMode(nodeOnly bool) string {
	if nodeOnly {
		return "node"
	}
	return "controller+node"
}

func run() error {
	// Parse endpoint
	scheme, addr, err := parseEndpoint(*endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint: %w", err)
	}

	// Remove existing socket file
	if scheme == "unix" {
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing socket: %w", err)
		}

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(addr), 0750); err != nil {
			return fmt.Errorf("failed to create socket directory: %w", err)
		}
	}

	// Create listener
	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s://%s: %w", scheme, addr, err)
	}
	defer listener.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(logGRPC),
	)

	// Create and register Identity service
	identitySvc := identity.New()
	csi.RegisterIdentityServer(grpcServer, identitySvc)
	klog.Info("Identity service registered")

	// Create and register Node service
	nodeSvc := node.New(*nodeID)
	csi.RegisterNodeServer(grpcServer, nodeSvc)
	klog.Info("Node service registered")

	// Create and register Controller service (if not node-only mode)
	if !*nodeOnly {
		controllerSvc := controller.New(*nodeID)
		csi.RegisterControllerServer(grpcServer, controllerSvc)
		klog.Info("Controller service registered")

		// Initialize controller (no-op now, but kept for compatibility)
		if err := controllerSvc.Setup(); err != nil {
			klog.Warningf("Controller setup warning: %v", err)
		}
	}

	// Handle shutdown gracefully
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopCh
		klog.Info("Received shutdown signal, stopping server...")
		grpcServer.GracefulStop()
	}()

	// Start serving
	klog.Infof("Server listening on %s://%s", scheme, addr)
	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

func parseEndpoint(endpoint string) (string, string, error) {
	// Expected format: unix:///path/to/socket or tcp://host:port
	if len(endpoint) < 7 {
		return "", "", fmt.Errorf("invalid endpoint format: %s", endpoint)
	}

	scheme := ""
	addr := ""

	if endpoint[:7] == "unix://" {
		scheme = "unix"
		addr = endpoint[7:]
	} else if endpoint[:6] == "tcp://" {
		scheme = "tcp"
		addr = endpoint[6:]
	} else {
		return "", "", fmt.Errorf("unsupported endpoint scheme: %s", endpoint)
	}

	return scheme, addr, nil
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.V(5).Infof("GRPC call: %s", info.FullMethod)
	klog.V(6).Infof("GRPC request: %+v", req)

	resp, err := handler(ctx, req)

	if err != nil {
		klog.Warningf("GRPC error: %s returned error: %v", info.FullMethod, err)
	} else {
		klog.V(6).Infof("GRPC response: %+v", resp)
	}

	return resp, err
}
