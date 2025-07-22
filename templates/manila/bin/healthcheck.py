#!/usr/bin/env python3
#
# Copyright 2023 Red Hat Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.

# Trivial HTTP server to check health of scheduler, and share services.
# Manila-API has its own health check endpoint and does not need this.
#
# The only check this server currently does is using the manila-manage
# service list command, which accesses DB directly and gets data related
# to the services already registered in Manila.
#
# Manila-manage will allow us to keep imports minimal and also check on
# what has already been registered in the manila-database, and with this
# we can checks if services are up or not.
#
# For share services all enabled backends must be up to return 200, so it is
# recommended to use a different pod for each backend to avoid one backend
# affecting others.
#
# Requires the name of the service as the first argument (share,
# scheduler) and optionally a second argument with the location of the
# configuration directory (defaults to /etc/manila/manila.conf.d)

from http import server
import signal
import socket
import subprocess
import sys
import time
import threading

from oslo_config import cfg

from manila.common import config as manila_config


SERVER_PORT = 8080
CONF = cfg.CONF
BINARIES = ('share', 'scheduler')
STATE_UP = ":-)"
STATUS_ENABLED = "enabled"

'''
manila-manage returns services using the following layout:

# SERVICE LAYOUT

| binary    | -> 0
| host      | -> 1
| zone      | -> 2
| status    | -> 3
| state     | -> 4
| udated_at | -> 5

The layout is defined in the upstream manila-manage code [1].
Every time a manila-manage command is performed, we can resolve the position
of each field, and we can easily decode into a dict.

[1] https://github.com/openstack/manila/blob/master/manila/cmd/manage.py#L376
'''


class HTTPServerV6(server.HTTPServer):
    address_family = socket.AF_INET6


class ServiceNotFoundException(Exception):

    def __init__(self, filters={}):
        self.message = "Service not found."
        if filters:
            self.message = f'Service not found with filters: {filters}'
        super(ServiceNotFoundException, self).__init__(self.message)


class ServiceNotUp(Exception):
    message = "Service error: Service is not UP."

    def __init__(self):
        super(ServiceNotUp, self).__init__(self.message)


class HeartbeatServer(server.BaseHTTPRequestHandler):
    @classmethod
    def initialize_class(cls, binary, config_dir):
        """Calculate and initialize constants"""
        cls.binary = 'manila-' + binary
        cls.config_dir = config_dir

        if binary != 'share':
            services_filters = [
                {'host': CONF.host,
                 'state': STATE_UP,
                 'status': STATUS_ENABLED,
                 'binary': cls.binary}]
        else:
            services_filters = []
            for backend in CONF.enabled_share_backends:
                host = "%s@%s" % (CONF.host, backend)
                services_filters.append({'host': host,
                                         'status': STATUS_ENABLED,
                                         'binary': cls.binary})

        # Store the filters used to find out services.
        cls.services_filters = services_filters

    @staticmethod
    def get_status(service):
        """
        Because status is either enabled or disabled, this function looks for
        it by iterating the array; by doing this it does not ave to rely on a
        particular position
        """
        return STATUS_ENABLED if STATUS_ENABLED in service else "disabled"

    @staticmethod
    def validate_line(service):
        """
        Process the line if there's at least one element
        """
        return False if ((len(service) - 1) <= 0) else True

    @staticmethod
    def get_state(service):
        """
        Because STATE_UP has a special symbol, the function does not need to
        search for it at a given position, but it can match it by iterating
        the array and return a negative result if the symbol is not found
        """
        return STATE_UP if STATE_UP in service else "XXX"

    @staticmethod
    def get_hostgroup(service):
        """
        As per the layout defined in the manila-manage command, hostgroup is
        always the second entry of the table
        """
        return service[1] if len(service) > 1 else None

    @staticmethod
    def get_service_name(service):
        """
        Service name is always the first entry of the table defined by the
        manila-manage command
        """
        return service[0] if service else None

    @staticmethod
    def check_service(services, **filters):
        # Check if services were already created and are registered in the db
        our_services = [service for service in services
                        if all(service.get(key, None) == value
                               for key, value in filters.items())]
        if not our_services:
            raise ServiceNotFoundException(filters=filters)

        num_services = len(our_services)
        if num_services != 1:
            print(f'{num_services} services found!: {our_services}')

        ok = all(service['state'] == STATE_UP for service in our_services)
        if not ok:
            raise ServiceNotUp()

    def _parse_service_list_output(self, services_list):
        # reuse the existing BINARIES tuple to resolve the allowed services
        # and prefix each item
        allowed_services = tuple('manila-{}'.format(item) for item in BINARIES)
        # Decode the characters, remove the line splits and the
        # blank spaces
        services_list = services_list.decode('utf-8').split('\n')
        services = []
        for service in services_list:
            # clean up tokens of the current line
            tokens = [token for token in service.split() if token]
            # invalid line: stop early and move to the next item, no need to
            # try to process each element
            if not self.validate_line(tokens):
                continue
            # First line is related to the headers. We don't care about the
            # updated at, so we ignore it
            if "Binary" in tokens:
                continue
            # Process a valid line for a given service and fill the dict
            service_name = self.get_service_name(tokens)
            if service_name and service_name in allowed_services:
                services.append({
                    'binary': service_name,
                    'host': self.get_hostgroup(tokens),
                    'status': self.get_status(tokens),
                    'state': self.get_state(tokens),
                })
        return services

    def do_GET(self):
        try:
            cmd = ['manila-manage', '--config-dir', self.config_dir,
                   'service', 'list']
            services_list = subprocess.run(cmd, stdout=subprocess.PIPE)

            services_list = services_list.stdout
        except Exception as exc:
            return self.send_error(
                500,
                'Failed to get existing services in the Manila ',
                f'database: {exc}')

        try:
            services = self._parse_service_list_output(services_list)
        except ServiceNotFoundException as e:
            return self.send_error(500, e.message)

        for service_filters in self.services_filters:
            try:
                self.check_service(services, **service_filters)
            except Exception as exc:
                return self.send_error(500, exc.args[0])

        self.send_response(200)
        self.send_header("Content-type", "text/html")
        self.end_headers()
        self.wfile.write('<html><body>OK</body></html>'.encode('utf-8'))


def get_stopper(server):
    def stopper(signal_number=None, frame=None):
        print("Stopping server.")
        server.shutdown()
        server.server_close()
        print("Server stopped.")
        sys.exit(0)
    return stopper


if __name__ == "__main__":
    if 3 < len(sys.argv) < 2 or sys.argv[1] not in BINARIES:
        print('Healthcheck requires the binary type as argument (one of: '
              + ', '.join(BINARIES) +
              ') and optionally the location of the config directory.')
        sys.exit(1)
    binary = sys.argv[1]

    cfg_dir = (
        sys.argv[2] if len(sys.argv) == 3 else '/etc/manila/manila.conf.d')

    # Due to our imports manila.common.config.global_opts are registered in
    # manila/share/__init__.py bringing in the following config options we
    # want: host, enabled_backends
    # Initialize Oslo Config
    CONF.register_opts(manila_config.global_opts)
    CONF(['--config-dir', cfg_dir], project='manila')

    HeartbeatServer.initialize_class(binary, cfg_dir)

    hostname = socket.gethostname()
    ipv6_address = socket.getaddrinfo(hostname, None, socket.AF_INET6)
    if ipv6_address:
        webServer = HTTPServerV6(("::", SERVER_PORT), HeartbeatServer)
    else:
        webServer = server.HTTPServer(("0.0.0.0", SERVER_PORT), HeartbeatServer)
    stop = get_stopper(webServer)

    # Need to run the server on a different thread because its shutdown method
    # will block if called from the same thread, and the signal handler must be
    # on the main thread in Python.
    thread = threading.Thread(target=webServer.serve_forever)
    thread.daemon = True
    thread.start()
    print(f"Manila Healthcheck Server started http://{hostname}:{SERVER_PORT}")
    signal.signal(signal.SIGTERM, stop)

    try:
        while True:
            time.sleep(60)
    except KeyboardInterrupt:
        pass
    finally:
        stop()
