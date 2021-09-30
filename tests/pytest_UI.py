import logging.config
import http.cookiejar
import random
import string

import mechanize
import yaml
import pytest


@pytest.fixture(scope="function")
def browser():
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
    b.set_debug_http(True)
    return b


@pytest.fixture
def config():
    cfg = yaml.load(open('conf/config.yml', 'r'))
    return cfg


@pytest.fixture()
def admin(browser, config):
    auth = (c := config['core'])['adminUser'], c['adminPass']
    return _login(browser, auth)

def _create_user(admin, values):
    b = admin
    b.follow_link(text="User Management")
    b.follow_link(predicate=has_attr('Add a user'))

    # FIXME name form
    b.select_form(predicate=lambda x: x.method == 'post')
    for k, v in values.items():
        b.form.set_value(v, k)
    b.submit()
    alert = b._factory.root.findall('body/div/div[@role="alert"]')
    assert len(alert) == 1 and alert[0].text.strip() == "user created successfully"
    return values["email"],values["password"]

def _destroy_user(admin, uid):
    b = admin
    b.follow_link(text="User Management")
    for user in b._factory.root.findall('body/div/div/table[@id="userTable"]/tbody/'):
        email,*_ = list(map(lambda x: x.text.strip() if x.text else '', list(user)))
        if email == uid:
            break
    else:
        assert False
    a = user.findall('td/a[@title="Edit user"]')
    assert len(a) == 1
    b.follow_link(url=a[0].attrib['href'])

    # FIXME name form
    b.select_form(predicate=lambda x: x.method == 'post')
    disabled = b.find_control("isdisabled")
    disabled.set_single("true")
    b.submit()

def _destroy_peer(admin, uid):
    b = admin
    b.follow_link(text="Administration")
    peers = b._factory.root.findall('body/div/div/table[@id="userTable"]/tbody/tr')
    for idx,peer in enumerate(peers):
        if idx % 2 == 1:
            continue
        head, Identifier, PublicKey, EMail, IPs, Handshake, tail = list(map(lambda x: x.text.strip() if x.text else x, list(peer)))
        print(Identifier)
        if EMail != uid:
            continue
        peer = peers[idx+1]
        a = peer.findall('.//a[@title="Delete peer"]')
        assert len(a) == 1
        b.follow_link(url=a[0].attrib['href'])


def _list_peers(user):
    r = []
    b = user
    b.follow_link(predicate=has_attr('User-Profile'))
    profiles = b._factory.root.findall('body/div/div/table[@id="userTable"]/tbody/tr')
    for idx,profile in enumerate(profiles):
        if idx % 2 == 1:
            continue
        head, Identifier, PublicKey, EMail, IPs, Handshake = list(map(lambda x: x.text.strip() if x.text else x, list(profile)))
        profile = profiles[idx+1]
        pre = profile.findall('.//pre')
        assert len(pre) == 1
        r.append((PublicKey, pre))
    return r


@pytest.fixture(scope="session")
def user_data():
    values = {
        "email": f"test+{randstr()}@example.org",
        "password": randstr(12),
        "firstname": randstr(8),
        "lastname": randstr(12)
    }
    return values

@pytest.fixture
def user(admin, user_data, config):
    b = admin
    auth = _create_user(b, user_data)
    _logout(b)
    _login(b, auth)
    assert b.find_link(predicate=has_attr('User-Profile'))
    yield b
    _logout(b)
    auth = (c := config['core'])['adminUser'], c['adminPass']
    _login(b, auth)
    _destroy_user(b, user_data["email"])
    _destroy_peer(b, user_data["email"])

@pytest.fixture
def peer(admin, user, user_data):
    pass

def _login(browser, auth):
    b = browser
    b.open("http://localhost:8123/")

    b.follow_link(text="Login")

    b.select_form(name="login")
    username, password = auth
    b.form.set_value(username, "username")
    b.form.set_value(password, "password")
    b.submit()
    return b

def _logout(browser):
    browser.follow_link(text="Logout")
    return browser

def has_attr(value, attr='title'):
    def find_attr(x):
        return any([a == (attr, value) for a in x.attrs])
    return find_attr


def _server(browser, addr):
    b = browser
    b.follow_link(text="Administration")
    b.follow_link(predicate=has_attr('Edit interface settings'))
    b.select_form("server")

    values = {
        "displayname": "example0",
        "endpoint": "wg.example.org:51280",
        "ip": addr
    }
    for k, v in values.items():
        b.form.set_value(v, k)

    b.submit()
    return b

@pytest.fixture
def server(admin):
    return _server(admin, "10.0.0.0/24")

def randstr(l=6):
    return ''.join([random.choice(string.ascii_lowercase + string.digits) for i in range(l)])


def test_admin_login(admin):
    b = admin
    b.find_link("Administration")


def test_admin_server(admin):
    ip = "10.0.0.0/28"
    b = _server(admin, ip)
    b.select_form("server")
    assert ip == b.form.get_value("ip")


def test_admin_create_peer(server, user_data):
    auth = _create_user(server, user_data)


def test_admin_create_user(admin, user_data):
    auth = _create_user(admin, user_data)


def test_user_login(server, user):
    b = user
    b.follow_link(predicate=has_attr('User-Profile'))

def test_user_config(server, user):
    b = user
    peers = _list_peers(b)
    assert len(peers) >= 1