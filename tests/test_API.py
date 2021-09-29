import ipaddress
import collections
import string
import unittest
import datetime
import re
import uuid
import subprocess
import random

import logging
import logging.config

import mechanize

from pyswagger import App, Security
from pyswagger.contrib.client.requests import Client


log = logging.getLogger("api")

class HttpFormatter(logging.Formatter):

    def _formatHeaders(self, d):
        return '\n'.join(f'{k}: {v}' for k, v in d.items())

    def formatMessage(self, record):
        result = super().formatMessage(record)
        if record.name == 'api':
            result += '''
---------------- request ----------------
{req.method} {req.url}
{reqhdrs}

{req.body}
---------------- response ----------------
{res.status_code} {res.reason} {res.url}
{reshdrs}

{res.text}
---------------- end ----------------
'''.format(req=record.req, res=record.res, reqhdrs=self._formatHeaders(record.req.headers),
                reshdrs=self._formatHeaders(record.res.headers), )

        return result


logging.config.dictConfig(
    {
        "version": 1,
        "formatters": {
            "http": {
                "()": HttpFormatter,
                "format": "{asctime} {levelname} {name} {message}",
                "style":'{',
            },
            "detailed": {
                "class": "logging.Formatter",
                "format": "%(asctime)s %(name)-9s %(levelname)-4s %(message)s",
            },
            "plain": {
                "class": "logging.Formatter",
                "format": "%(message)s",
            }
        },
        "handlers": {
            "console": {
                "class": "logging.StreamHandler",
                "level": "DEBUG",
                "formatter": "detailed",
            },
            "console_http": {
                "class": "logging.StreamHandler",
                "level": "DEBUG",
                "formatter": "http",
            },
        },
        "root": {
            "level": "DEBUG",
            "handlers": ["console"],
            "propagate": True
        },
        'loggers': {
            'api': {
                "level": "INFO",
                "handlers": ["console_http"]
            },
            "requests.packages.urllib3": {
                "level": "DEBUG",
                "handlers": ["console"],
                "propagate": True
            },
        },
    }
)

log = logging.getLogger("api")

class ApiError(Exception):
    pass



def logHttp(response, *args, **kwargs):
    extra = {'req': response.request, 'res': response}
    log.debug('HTTP', extra=extra)

class WGPClient:
    def __init__(self, url, *auths):
        app = App._create_(url)
        auth = Security(app)
        for t, cred in auths:
            auth.update_with(t, cred)

        client = Client(auth)
        self.app, self.client = app, client

        self.client._Client__s.hooks['response'] = logHttp

    def call(self, name, **kwargs):
        #        print(f"{name} {kwargs}")
        op = self.app.op[name]
        req, resp = op(**kwargs)
        now = datetime.datetime.now()
        resp = self.client.request((req, resp))
        then = datetime.datetime.now()
        delta = then - now
        #        print(f"{resp.status} {delta}")

        if 200 <= resp.status <= 299:
            pass
        elif 400 <= resp.status <= 499:
            raise ApiError(resp.data["Message"])
        elif 500 == resp.status:
            raise ValueError(resp.data["Message"])
        elif 501 == resp.status:
            raise NotImplementedError(resp.data["Message"])
        elif 502 <= resp.status <= 599:
            raise ApiError(resp.data["Message"])
        return resp

    def GetDevice(self, **kwargs):
        return self.call("GetDevice", **kwargs).data

    def PatchDevice(self, **kwargs):
        return self.call("PatchDevice", **kwargs).data

    def PutDevice(self, **kwargs):
        return self.call("PutDevice", **kwargs).data

    def GetDevices(self, **kwargs):
        # FIXME - could return empty list?
        return self.call("GetDevices", **kwargs).data or []

    def DeletePeer(self, **kwargs):
        return self.call("DeletePeer", **kwargs).data

    def GetPeer(self, **kwargs):
        return self.call("GetPeer", **kwargs).data

    def PatchPeer(self, **kwargs):
        return self.call("PatchPeer", **kwargs).data

    def PostPeer(self, **kwargs):
        return self.call("PostPeer", **kwargs).data

    def PutPeer(self, **kwargs):
        return self.call("PutPeer", **kwargs).data

    def GetPeerDeploymentConfig(self, **kwargs):
        return self.call("GetPeerDeploymentConfig", **kwargs).data

    def PostPeerDeploymentConfig(self, **kwargs):
        return self.call("PostPeerDeploymentConfig", **kwargs).raw

    def GetPeerDeploymentInformation(self, **kwargs):
        return self.call("GetPeerDeploymentInformation", **kwargs).data

    def GetPeers(self, **kwargs):
        return self.call("GetPeers", **kwargs).data

    def DeleteUser(self, **kwargs):
        return self.call("DeleteUser", **kwargs).data

    def GetUser(self, **kwargs):
        return self.call("GetUser", **kwargs).data

    def PatchUser(self, **kwargs):
        return self.call("PatchUser", **kwargs).data

    def PostUser(self, **kwargs):
        return self.call("PostUser", **kwargs).data

    def PutUser(self, **kwargs):
        return self.call("PutUser", **kwargs).data

    def GetUsers(self, **kwargs):
        return self.call("GetUsers", **kwargs).data


def generate_wireguard_keys():
    """
    Generate a WireGuard private & public key
    Requires that the 'wg' command is available on PATH
    Returns (private_key, public_key), both strings
    """
    privkey = subprocess.check_output("wg genkey", shell=True).decode("utf-8").strip()
    pubkey = subprocess.check_output(f"echo '{privkey}' | wg pubkey", shell=True).decode("utf-8").strip()
    return (privkey, pubkey)


KeyTuple = collections.namedtuple("Keys", "private public")


class TestAPI(unittest.TestCase):
    URL = 'http://localhost:8123/swagger/doc.json'
    AUTH = {
        "api": ('ApiBasicAuth', ("wg@example.org", "abadchoice")),
        "general": ('GeneralBasicAuth', ("wg@example.org", "abadchoice"))
    }
    DEVICE = "wg-example0"
    IFADDR = "10.17.0.0/24"
    log = logging.getLogger("TestAPI")


    def _client(self, *auth):
        auth = ["general"] if auth is None else auth
        self.c = WGPClient(self.URL, *[self.AUTH[i] for i in auth])

    @property
    def randmail(self):
        return 'test+' + ''.join(
            [random.choice(string.ascii_lowercase + string.digits) for i in range(6)]) + '@example.org'

    @classmethod
    def setUpClass(cls) -> None:
        cls.finishInstallation()

    @classmethod
    def finishInstallation(cls) -> None:
        import http.cookiejar

        # Fake Cookie Policy to send the Secure cookies via http
        class InSecureCookiePolicy(http.cookiejar.DefaultCookiePolicy):
            def set_ok(self, cookie, request):
                return True

            def return_ok(self, cookie, request):
                return True

            def domain_return_ok(self, domain, request):
                return True

            def path_return_ok(self, path, request):
                return True

        b = mechanize.Browser()
        b.set_cookiejar(http.cookiejar.CookieJar(InSecureCookiePolicy()))
        b.set_handle_robots(False)
        b.open("http://localhost:8123/")

        b.follow_link(text="Login")

        b.select_form(name="login")
        username, password = cls.AUTH['api'][1]
        b.form.set_value(username, "username")
        b.form.set_value(password, "password")

        b.submit()

        b.follow_link(text="Administration")
        b.follow_link(predicate=lambda x: any([a == ('title', 'Edit interface settings') for a in x.attrs]))
        b.select_form("server")

        values = {
            "displayname": "example0",
            "endpoint": "wg.example.org:51280",
            "ip": cls.IFADDR
        }
        for k, v in values.items():
            b.form.set_value(v, k)

        b.submit()

        b.select_form("server")
#        cls.log.debug(b.form.get_value("ip"))

    def setUp(self) -> None:
        self._client('api')
        self.user = self.randmail

        # create a user …
        self.c.PostUser(User={"Firstname": "Test", "Lastname": "User", "Email": self.user})

        self.keys = KeyTuple(*generate_wireguard_keys())


    def _test_generate(self):
        def key_of(op):
            a, *b = list(filter(lambda x: len(x), re.split("([A-Z][a-z]+)", op.operationId)))
            return ''.join(b), a

        for op in sorted(self.c.app.op.values(), key=key_of):
            print(f"""
        def {op.operationId}(self, **kwargs):
            return self. call("{op.operationId}", **kwargs)
                """)

    def test_ops(self):
        for op in sorted(self.c.app.op.values(), key=lambda op: op.operationId):
            self.assertTrue(hasattr(self.c, op.operationId), f"{op.operationId} is missing")

    def test_Device(self):
        # FIXME device has to be completed via webif to be valid before it can be used via API
        devices = self.c.GetDevices()
        self.assertTrue(len(devices) > 0)

        for device in devices:
            dev = self.c.GetDevice(DeviceName=device.DeviceName)
            new = self.c.PutDevice(DeviceName=dev.DeviceName,
                                   Device={
                                       "DeviceName": dev.DeviceName,
                                       "IPsStr": dev.IPsStr,
                                       "PrivateKey": dev.PrivateKey,
                                       "Type": "client",
                                       "PublicKey": dev.PublicKey}
                                   )
            new = self.c.PatchDevice(DeviceName=dev.DeviceName,
                                     Device={
                                         "DeviceName": dev.DeviceName,
                                         "IPsStr": dev.IPsStr,
                                         "PrivateKey": dev.PrivateKey,
                                         "Type": "client",
                                         "PublicKey": dev.PublicKey}
                                     )

    def easy_peer(self):
        data = self.c.PostPeerDeploymentConfig(ProvisioningRequest={"Email": self.user, "Identifier": "debug"})
        data = data.decode()
        pubkey = re.search("# -WGP- PublicKey: (?P<pubkey>[^\n]+)\n", data, re.MULTILINE)['pubkey']
        privkey = re.search("PrivateKey = (?P<key>[^\n]+)\n", data, re.MULTILINE)['key']
        self.keys = KeyTuple(privkey, pubkey)

    def test_Peers(self):

        privkey, pubkey = generate_wireguard_keys()
        peer = {"UID": uuid.uuid4().hex,
                "Identifier": uuid.uuid4().hex,
                "DeviceName": self.DEVICE,
                "PublicKey": pubkey,
                "DeviceType": "client",
                "IPsStr": str(self.IFADDR),
                "Email": self.user}

        # keypair is created server side if private key is not submitted
        with self.assertRaisesRegex(ApiError, "peer not found"):
            self.c.PostPeer(DeviceName=self.DEVICE, Peer=peer)

        # create
        peer["PrivateKey"] = privkey
        p = self.c.PostPeer(DeviceName=self.DEVICE, Peer=peer)
        self.assertListEqual([p.PrivateKey, p.PublicKey], [privkey, pubkey])

        # lookup created peer
        for p in self.c.GetPeers(DeviceName=self.DEVICE):
            if pubkey == p.PublicKey:
                break
        else:
            self.assertTrue(False)

        # get
        gp = self.c.GetPeer(PublicKey=p.PublicKey)
        self.assertListEqual([gp.PrivateKey, gp.PublicKey], [p.PrivateKey, p.PublicKey])

        # change?
        peer['Identifier'] = 'changed'
        n = self.c.PatchPeer(PublicKey=p.PublicKey, Peer=peer)
        self.assertListEqual([n.PrivateKey, n.PublicKey], [privkey, pubkey])

        # change ?
        peer['Identifier'] = 'changedagain'
        n = self.c.PutPeer(PublicKey=p.PublicKey, Peer=peer)
        self.assertListEqual([n.PrivateKey, n.PublicKey], [privkey, pubkey])

        # invalid change operations
        n = peer.copy()
        n['PrivateKey'], n['PublicKey'] = generate_wireguard_keys()
        with self.assertRaisesRegex(ApiError, "PublicKey parameter must match the model public key"):
            self.c.PutPeer(PublicKey=p.PublicKey, Peer=n)

        with self.assertRaisesRegex(ApiError, "PublicKey parameter must match the model public key"):
            self.c.PatchPeer(PublicKey=p.PublicKey, Peer=n)

        n = self.c.DeletePeer(PublicKey=p.PublicKey)

    def test_Deployment(self):
        log.setLevel(logging.DEBUG)
        self._client("general")
        self.easy_peer()

        self.c.GetPeerDeploymentConfig(PublicKey=self.keys.public)
        self.c.GetPeerDeploymentInformation(Email=self.user)
        log.setLevel(logging.INFO)

    def test_User(self):
        u = self.c.PostUser(User={"Firstname": "Test", "Lastname": "User", "Email": self.randmail})
        for i in self.c.GetUsers():
            if i.Email == u.Email:
                break
        else:
            self.assertTrue(False)

        u = self.c.GetUser(Email=u.Email)
        self.c.PutUser(Email=u.Email, User={"Firstname": "Test", "Lastname": "User", "Email": u.Email})
        self.c.PatchUser(Email=u.Email, User={"Firstname": "Test", "Lastname": "User", "Email": u.Email})

        # list a deleted user
        self.c.DeleteUser(Email=u.Email)

        for i in self.c.GetUsers():
            break


    def _clear_peers(self):
        for p in self.c.GetPeers(DeviceName=self.DEVICE):
            self.c.DeletePeer(PublicKey=p.PublicKey)

    def _clear_users(self):
        for p in self.c.GetUsers():
            if p.Email == self.AUTH['api'][1][0]:
                continue
            self.c.DeleteUser(Email=p.Email)


    def _createPeer(self):
        privkey, pubkey = generate_wireguard_keys()
        peer = {"UID": uuid.uuid4().hex,
                "Identifier": uuid.uuid4().hex,
                "DeviceName": self.DEVICE,
                "PublicKey": pubkey,
                "PrivateKey": privkey,
                "DeviceType": "client",
                #                    "IPsStr": str(self.ifaddr),
                "Email": self.user}
        self.c.PostPeer(DeviceName=self.DEVICE, Peer=peer)
        return pubkey

    def test_address_exhaustion(self):
        global log
        self._clear_peers()
        self._clear_users()

        self.NETWORK = ipaddress.ip_network("10.0.0.0/29")
        addr = ipaddress.ip_address(
            random.randrange(int(self.NETWORK.network_address) + 1, int(self.NETWORK.broadcast_address) - 1))
        self.__class__.IFADDR = str(ipaddress.ip_interface(f"{addr}/{self.NETWORK.prefixlen}"))

        # reconfigure via web ui - set the ifaddr with less addrs in pool
        self.finishInstallation()

        keys = set()
        EADDRESSEXHAUSTED = "failed to get available IP addresses: no more available address from cidr"
        with self.assertRaisesRegex(ValueError, EADDRESSEXHAUSTED):
            for i in range(self.NETWORK.num_addresses + 1):
                keys.add(self._createPeer())

        n = keys.pop()
        self.c.DeletePeer(PublicKey=n)
        self._createPeer()

        with self.assertRaisesRegex(ValueError, EADDRESSEXHAUSTED):
            self._createPeer()

