#!/usr/bin/python3
import json
import os
import re
import ssl
import subprocess
import threading
import urllib.error
import urllib.request
from datetime import datetime

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
gi.require_version("LightDM", "1")
from gi.repository import Gdk, GLib, Gtk, LightDM


SERVER = os.environ.get("SWBADGE_SERVER", "https://login01.example.test:8080").rstrip("/")
CLIENT_ID = os.environ.get("SWBADGE_CLIENT_ID", "client01-greeter")
CA_FILE = os.environ.get("SWBADGE_CA_FILE", "/etc/stumpfworks-badge/stumpfworks-homelab-ca.crt")
CAMERA_HELPER = os.environ.get("SWBADGE_CAMERA_HELPER", "/usr/local/bin/sw-badge-camera-linux")
GRANT_FILE = os.environ.get("SWBADGE_GRANT_FILE", "/run/swbadge/login-grant")
DEVICE_NAME = os.environ.get("SWBADGE_DEVICE_NAME", os.uname().nodename.split(".")[0].upper())
PAYLOAD = re.compile(r"^SWBADGE:1:([^:]+):(.+)$")


class NativeGreeter(Gtk.Window):
    def __init__(self):
        super().__init__(title="StumpfWorks Identity")
        self.badge_id = ""
        self.token = ""
        self.scanning = False
        self.mode = "badge"
        self.auth_mode = ""
        self.network_online = False
        self.network_check_running = False
        self.wifi_dialog = None
        self.set_decorated(False)
        screen = Gdk.Screen.get_default()
        self.set_default_size(screen.get_width(), screen.get_height())
        self.move(0, 0)
        self.set_keep_above(True)
        self.connect("realize", self.ensure_pointer_visible)
        self.connect("destroy", Gtk.main_quit)
        self._style()
        self._build()
        self.greeter = LightDM.Greeter()
        self.greeter.connect("show-prompt", self.on_show_prompt)
        self.greeter.connect("show-message", self.on_show_message)
        self.greeter.connect("authentication-complete", self.on_authentication_complete)
        if not self.greeter.connect_to_daemon_sync():
            raise RuntimeError("Verbindung zu LightDM fehlgeschlagen")
        self.update_clock()
        GLib.timeout_add_seconds(1, self.update_clock)
        GLib.timeout_add(300, self.refresh_network)
        GLib.timeout_add_seconds(5, self.refresh_network)
        GLib.timeout_add(900, self.start_scan)

    def _style(self):
        css = b"""
        window { background: #080d15; color: #f2f7ff; }
        #shell { background-image: radial-gradient(circle farthest-corner at 50% 0%, #1b2d46, #080d15 70%); }
        #topbar { padding: 24px 34px; }
        #wordmark { color: #f2f7ff; font-size: 18px; font-weight: bold; letter-spacing: 2px; }
        #device { color: #8091a8; font-size: 13px; }
        #clock { color: #f2f7ff; font-size: 22px; font-weight: bold; }
        #date { color: #8091a8; font-size: 13px; }
        #card { background: #111a27; border: 1px solid #304660; border-radius: 26px; padding: 38px 52px; box-shadow: 0 22px 60px alpha(#000000, 0.45); }
        #brand { color: #4ee1a0; font-size: 13px; font-weight: bold; letter-spacing: 3px; }
        #title { color: #f2f7ff; font-size: 38px; font-weight: bold; }
        #icon { color: #4ee1a0; background: alpha(#4ee1a0, 0.08); border: 1px solid alpha(#4ee1a0, 0.35); border-radius: 60px; min-width: 112px; min-height: 112px; font-size: 62px; font-weight: bold; }
        #icon.state-error { color: #ff6f88; background: alpha(#ff6f88, 0.08); border-color: alpha(#ff6f88, 0.4); }
        #icon.state-success { color: #4ee1a0; }
        #message { color: #a9b8cc; font-size: 18px; }
        #badge { color: #4ee1a0; font-family: monospace; font-size: 17px; }
        entry { background: #09111d; color: white; border: 1px solid #344861; border-radius: 11px; padding: 14px; font-size: 22px; }
        button { background-image: linear-gradient(to bottom, #5794ff, #3978e8); color: #ffffff; border: 1px solid #70a6ff; border-radius: 12px; padding: 14px 22px; font-size: 17px; font-weight: bold; box-shadow: 0 6px 18px alpha(#1d63d8, 0.28); }
        button:hover { background-image: linear-gradient(to bottom, #6ba2ff, #4a87f3); border-color: #9bc2ff; }
        button:active { background: #2f6ed9; box-shadow: none; }
        button:disabled { background: #233148; color: #718198; border-color: #344861; box-shadow: none; }
        #fallback { background: alpha(#0b1421, 0.55); color: #b7c5d8; border: 1px solid #40546d; box-shadow: none; font-weight: normal; }
        #fallback:hover { background: #192638; color: #ffffff; border-color: #607895; }
        #power { background: transparent; background-image: none; color: #9cacc0; border: 1px solid transparent; box-shadow: none; padding: 7px 13px; font-size: 15px; font-weight: normal; }
        #power:hover { background: alpha(#ffffff, 0.06); border-color: #344861; color: #ffffff; }
        #statusbar { padding: 10px 34px 12px 34px; }
        #status { color: #8294aa; font-size: 13px; }
        #online { color: #4ee1a0; font-size: 13px; }
        #offline { color: #ff6f88; font-size: 13px; }
        #network { background: alpha(#ffffff, 0.04); background-image: none; color: #b8c7da; border: 1px solid #40546d; box-shadow: none; padding: 7px 15px; font-size: 13px; font-weight: normal; }
        #network:hover { background: #1a2a3e; border-color: #5d7796; color: #ffffff; }
        #wifi-row { background: #0c1522; border: 1px solid #304660; padding: 10px; }
        """
        provider = Gtk.CssProvider()
        provider.load_from_data(css)
        Gtk.StyleContext.add_provider_for_screen(Gdk.Screen.get_default(), provider, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

    def _build(self):
        shell = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        shell.set_name("shell")
        shell.set_halign(Gtk.Align.FILL)
        shell.set_valign(Gtk.Align.FILL)
        self.add(shell)

        topbar = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL)
        topbar.set_name("topbar")
        brand_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        wordmark = Gtk.Label(label="STUMPFWORKS")
        wordmark.set_name("wordmark")
        wordmark.set_xalign(0)
        device = Gtk.Label(label="SECURE WORKSTATION  /  " + DEVICE_NAME)
        device.set_name("device")
        device.set_xalign(0)
        brand_box.pack_start(wordmark, False, False, 0)
        brand_box.pack_start(device, False, False, 0)
        topbar.pack_start(brand_box, False, False, 0)
        clock_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        self.clock = Gtk.Label()
        self.clock.set_name("clock")
        self.clock.set_xalign(1)
        self.date = Gtk.Label()
        self.date.set_name("date")
        self.date.set_xalign(1)
        clock_box.pack_start(self.clock, False, False, 0)
        clock_box.pack_start(self.date, False, False, 0)
        topbar.pack_end(clock_box, False, False, 0)
        shell.pack_start(topbar, False, False, 0)

        center = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        center.set_halign(Gtk.Align.CENTER)
        center.set_valign(Gtk.Align.CENTER)
        center.set_hexpand(True)
        center.set_vexpand(True)
        shell.pack_start(center, True, True, 0)

        card = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18)
        card.set_name("card")
        card.set_size_request(660, -1)
        card.set_halign(Gtk.Align.CENTER)
        card.set_valign(Gtk.Align.CENTER)
        center.pack_start(card, False, False, 0)

        brand = Gtk.Label(label="STUMPFWORKS / IDENTITY")
        brand.set_name("brand")
        card.pack_start(brand, False, False, 0)
        self.icon = Gtk.Label(label="▣")
        self.icon.set_name("icon")
        self.icon.set_halign(Gtk.Align.CENTER)
        card.pack_start(self.icon, False, False, 6)
        self.title_label = Gtk.Label(label="Badge Login")
        self.title_label.set_name("title")
        card.pack_start(self.title_label, False, False, 0)
        self.message = Gtk.Label(label="Kamera wird vorbereitet …")
        self.message.set_name("message")
        card.pack_start(self.message, False, False, 0)
        self.badge = Gtk.Label(label="")
        self.badge.set_name("badge")
        card.pack_start(self.badge, False, False, 0)

        self.pin_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        self.pin = Gtk.Entry()
        self.pin.set_visibility(False)
        self.pin.set_invisible_char("●")
        self.pin.set_placeholder_text("PIN")
        self.pin.set_input_purpose(Gtk.InputPurpose.DIGITS)
        self.pin.set_max_length(32)
        self.pin.connect("activate", self.submit_pin)
        self.pin_box.pack_start(self.pin, False, False, 0)
        pin_button = Gtk.Button(label="Anmelden")
        pin_button.connect("clicked", self.submit_pin)
        self.pin_box.pack_start(pin_button, False, False, 0)
        card.pack_start(self.pin_box, False, False, 0)
        self.pin_box.hide()

        self.retry = Gtk.Button(label="Erneut scannen")
        self.retry.connect("clicked", lambda _b: self.start_scan())
        card.pack_start(self.retry, False, False, 0)
        self.retry.hide()

        self.login_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        self.username = Gtk.Entry()
        self.username.set_placeholder_text("AD-Benutzername")
        self.username.connect("activate", self.begin_password_login)
        self.login_box.pack_start(self.username, False, False, 0)
        self.password = Gtk.Entry()
        self.password.set_visibility(False)
        self.password.set_invisible_char("●")
        self.password.set_placeholder_text("AD-Passwort")
        self.password.connect("activate", self.submit_password)
        self.login_box.pack_start(self.password, False, False, 0)
        self.password.hide()
        login_button = Gtk.Button(label="Mit AD anmelden")
        login_button.connect("clicked", self.password_action)
        self.login_box.pack_start(login_button, False, False, 0)
        self.sessions = Gtk.ComboBoxText()
        default_session = ""
        for session in LightDM.get_sessions():
            self.sessions.append(session.get_key(), session.get_name())
            if session.get_key() == "xfce":
                default_session = session.get_key()
        self.sessions.set_active_id(default_session or "lightdm-xsession")
        self.login_box.pack_start(self.sessions, False, False, 0)
        back = Gtk.Button(label="Zurück zum Badge")
        back.set_name("fallback")
        back.connect("clicked", self.badge_mode)
        self.login_box.pack_start(back, False, False, 0)
        card.pack_start(self.login_box, False, False, 0)
        self.login_box.hide()

        self.fallback = Gtk.Button(label="Mit AD-Passwort anmelden")
        self.fallback.set_name("fallback")
        self.fallback.connect("clicked", self.password_fallback)
        card.pack_end(self.fallback, False, False, 0)

        statusbar = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=18)
        statusbar.set_name("statusbar")
        self.network_status = Gtk.Label(label="●  NETZWERK WIRD GEPRÜFT")
        self.network_status.set_name("status")
        statusbar.pack_start(self.network_status, False, False, 0)
        self.server_status = Gtk.Label(label="LOGIN01: …")
        self.server_status.set_name("status")
        statusbar.pack_start(self.server_status, False, False, 0)
        self.network_button = Gtk.Button(label="Netzwerk")
        self.network_button.set_name("network")
        self.network_button.connect("clicked", self.open_network_dialog)
        statusbar.pack_start(self.network_button, False, False, 0)
        power = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        restart = Gtk.Button(label="Neu starten")
        restart.set_name("power")
        restart.connect("clicked", lambda _b: self.power_action("restart"))
        shutdown = Gtk.Button(label="Herunterfahren")
        shutdown.set_name("power")
        shutdown.connect("clicked", lambda _b: self.power_action("shutdown"))
        power.pack_start(restart, False, False, 0)
        power.pack_start(shutdown, False, False, 0)
        statusbar.pack_end(power, False, False, 0)
        shell.pack_end(statusbar, False, False, 0)

    def show_state(self, title, message, icon="▣", badge="", pin=False, retry=False):
        self.title_label.set_text(title)
        self.message.set_text(message)
        self.icon.set_text(icon)
        icon_style = self.icon.get_style_context()
        icon_style.remove_class("state-error")
        icon_style.remove_class("state-success")
        if icon == "×":
            icon_style.add_class("state-error")
        elif icon == "✓":
            icon_style.add_class("state-success")
        self.badge.set_text(badge)
        self.pin_box.set_visible(pin)
        self.retry.set_visible(retry)
        self.login_box.hide()
        self.fallback.show()
        if pin:
            self.pin.set_text("")
            self.pin.grab_focus()

    def update_clock(self):
        now = datetime.now()
        self.clock.set_text(now.strftime("%H:%M"))
        self.date.set_text(now.strftime("%A, %d. %B %Y"))
        return True

    def ensure_pointer_visible(self, _widget):
        display = Gdk.Display.get_default()
        window = self.get_window()
        if display and window:
            window.set_cursor(Gdk.Cursor.new_for_display(display, Gdk.CursorType.LEFT_PTR))
            display.flush()

    def refresh_network(self):
        if not self.network_check_running:
            self.network_check_running = True
            threading.Thread(target=self._read_network_state, daemon=True).start()
        return True

    def refresh_network_once(self):
        self.refresh_network()
        return False

    def _run_nmcli(self, args, timeout=12, input_text=None):
        return subprocess.run(
            ["nmcli", "--colors", "no"] + args,
            input=input_text,
            text=True,
            capture_output=True,
            timeout=timeout,
            check=True,
            env={**os.environ, "LC_ALL": "C"},
        ).stdout

    def _read_network_state(self):
        online = False
        summary = "Nicht verbunden"
        wifi_available = False
        try:
            output = self._run_nmcli(["-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status"])
            active = []
            for line in output.splitlines():
                fields = self._split_nmcli(line)
                if len(fields) < 4:
                    continue
                _device, kind, state, connection = fields[:4]
                if kind == "wifi":
                    wifi_available = True
                if state in ("connected", "connected (externally)") and kind in ("wifi", "ethernet"):
                    active.append((kind, connection))
            if active:
                online = True
                kind, connection = active[0]
                summary = ("WLAN: " if kind == "wifi" else "LAN: ") + (connection or "verbunden")
        except Exception:
            summary = "NetworkManager nicht verfügbar"

        server_ok = False
        if online:
            try:
                ctx = ssl.create_default_context(cafile=CA_FILE)
                ctx.verify_flags &= ~ssl.VERIFY_X509_STRICT
                with urllib.request.urlopen(SERVER + "/api/v1/health", context=ctx, timeout=4) as response:
                    server_ok = response.status == 200
            except Exception:
                pass
        GLib.idle_add(self._apply_network_state, online, summary, wifi_available, server_ok)

    def _apply_network_state(self, online, summary, wifi_available, server_ok):
        was_online = self.network_online
        self.network_online = online
        self.network_check_running = False
        self.network_status.set_text(("●  " if online else "×  ") + summary.upper())
        self.network_status.set_name("online" if online else "offline")
        self.server_status.set_text("LOGIN01: ERREICHBAR" if server_ok else "LOGIN01: NICHT ERREICHBAR")
        self.server_status.set_name("online" if server_ok else "offline")
        self.network_button.set_visible(wifi_available)
        if online and not was_online and self.mode == "badge" and not self.scanning and not self.pin_box.get_visible():
            self.start_scan()
        return False

    @staticmethod
    def _split_nmcli(line):
        fields, current, escaped = [], [], False
        for char in line:
            if escaped:
                current.append(char)
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == ":":
                fields.append("".join(current))
                current = []
            else:
                current.append(char)
        fields.append("".join(current))
        return fields

    def open_network_dialog(self, _button):
        if self.wifi_dialog:
            self.wifi_dialog.present()
            return
        dialog = Gtk.Dialog(title="WLAN auswählen", transient_for=self, modal=True)
        dialog.set_default_size(560, 480)
        dialog.add_button("Schließen", Gtk.ResponseType.CLOSE)
        content = dialog.get_content_area()
        content.set_spacing(12)
        content.set_border_width(18)
        info = Gtk.Label(label="Verfügbare WLAN-Netze werden gesucht …")
        info.set_xalign(0)
        content.pack_start(info, False, False, 0)
        network_list = Gtk.ListBox()
        network_list.set_selection_mode(Gtk.SelectionMode.NONE)
        scroll = Gtk.ScrolledWindow()
        scroll.set_policy(Gtk.PolicyType.NEVER, Gtk.PolicyType.AUTOMATIC)
        scroll.add(network_list)
        content.pack_start(scroll, True, True, 0)
        dialog.connect("response", lambda widget, _response: widget.destroy())
        dialog.connect("destroy", lambda _widget: setattr(self, "wifi_dialog", None))
        self.wifi_dialog = dialog
        dialog.show_all()
        threading.Thread(target=self._scan_wifi, args=(network_list, info), daemon=True).start()

    def _scan_wifi(self, network_list, info):
        try:
            output = self._run_nmcli(["-t", "-f", "IN-USE,SSID,SIGNAL,SECURITY", "device", "wifi", "list", "--rescan", "yes"], timeout=20)
            networks = []
            seen = set()
            for line in output.splitlines():
                fields = self._split_nmcli(line)
                if len(fields) < 4 or not fields[1] or fields[1] in seen:
                    continue
                seen.add(fields[1])
                networks.append(tuple(fields[:4]))
            networks.sort(key=lambda item: (item[0] != "*", -int(item[2] or 0)))
            GLib.idle_add(self._fill_wifi_list, network_list, info, networks)
        except Exception:
            GLib.idle_add(info.set_text, "WLAN-Suche ist momentan nicht verfügbar.")

    def _fill_wifi_list(self, network_list, info, networks):
        info.set_text("WLAN auswählen" if networks else "Keine WLAN-Netze gefunden.")
        for active, ssid, signal, security in networks:
            row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=12)
            row.set_name("wifi-row")
            label = Gtk.Label(label=("●  " if active == "*" else "") + ssid + "  ·  " + signal + "%  ·  " + (security or "Offen"))
            label.set_xalign(0)
            label.set_hexpand(True)
            button = Gtk.Button(label="Verbunden" if active == "*" else "Verbinden")
            button.set_sensitive(active != "*")
            button.connect("clicked", self._ask_wifi_password, ssid, not security or security == "--", info)
            row.pack_start(label, True, True, 0)
            row.pack_end(button, False, False, 0)
            network_list.add(row)
        network_list.show_all()
        return False

    def _ask_wifi_password(self, _button, ssid, is_open, info):
        password = ""
        if not is_open:
            prompt = Gtk.Dialog(title="Mit WLAN verbinden", transient_for=self.wifi_dialog, modal=True)
            prompt.add_buttons("Abbrechen", Gtk.ResponseType.CANCEL, "Verbinden", Gtk.ResponseType.OK)
            box = prompt.get_content_area()
            box.set_spacing(10)
            box.set_border_width(18)
            box.pack_start(Gtk.Label(label=ssid), False, False, 0)
            entry = Gtk.Entry()
            entry.set_visibility(False)
            entry.set_invisible_char("●")
            entry.set_placeholder_text("WLAN-Passwort")
            entry.set_activates_default(True)
            prompt.set_default_response(Gtk.ResponseType.OK)
            box.pack_start(entry, False, False, 0)
            prompt.show_all()
            response = prompt.run()
            password = entry.get_text()
            entry.set_text("")
            prompt.destroy()
            if response != Gtk.ResponseType.OK or not password:
                return
        info.set_text("Verbindung zu „" + ssid + "“ wird hergestellt …")
        threading.Thread(target=self._connect_wifi, args=(ssid, password, info), daemon=True).start()

    def _connect_wifi(self, ssid, password, info):
        try:
            # NetworkManager reuses a matching saved profile when possible and
            # creates one otherwise. The password is supplied on stdin only.
            args = ["--ask", "device", "wifi", "connect", ssid]
            self._run_nmcli(args, timeout=35, input_text=(password + "\n") if password else None)
            GLib.idle_add(info.set_text, "Mit „" + ssid + "“ verbunden.")
            GLib.idle_add(self.refresh_network_once)
        except Exception:
            GLib.idle_add(info.set_text, "Verbindung fehlgeschlagen. Passwort und Signal prüfen.")
        finally:
            password = ""

    def start_scan(self):
        if self.scanning:
            return False
        if not self.network_online:
            self.mode = "badge"
            self.show_state("Netzwerk erforderlich", "LAN verbinden oder über „Netzwerk“ ein WLAN auswählen.", "×")
            return False
        self.mode = "badge"
        self.scanning = True
        self.badge_id = self.token = ""
        self.show_state("Badge Login", "Badge vor die Kamera halten …")
        threading.Thread(target=self._scan, daemon=True).start()
        return False

    def _scan(self):
        env = os.environ.copy()
        env["SWBADGE_CAMERA_PREVIEW"] = "0"
        try:
            result = subprocess.run([CAMERA_HELPER], env=env, text=True, capture_output=True, timeout=90, check=True)
            match = PAYLOAD.match(result.stdout.strip())
            if not match:
                raise RuntimeError("Ungültiger Badge")
            self.badge_id, self.token = match.group(1), match.group(2)
            GLib.idle_add(self._after_scan)
        except Exception:
            GLib.idle_add(self._scan_failed)

    def _after_scan(self):
        self.scanning = False
        if self.mode != "badge":
            return False
        self.show_state("Badge erkannt", "Berechtigung wird geprüft …", "✓", self.badge_id)
        threading.Thread(target=self._authorize, args=("",), daemon=True).start()
        return False

    def _scan_failed(self):
        self.scanning = False
        if self.mode != "badge":
            return False
        self.show_state("Scan fehlgeschlagen", "Badge konnte nicht gelesen werden.", "×", retry=True)
        return False

    def submit_pin(self, _widget):
        value = self.pin.get_text()
        self.pin.set_text("")
        self.pin_box.hide()
        self.show_state("Anmeldung", "PIN wird geprüft …", "▣", self.badge_id)
        threading.Thread(target=self._authorize, args=(value,), daemon=True).start()

    def _authorize(self, pin):
        payload = json.dumps({"badge_id": self.badge_id, "token": self.token, "client_id": CLIENT_ID, "pin": pin}).encode()
        try:
            ctx = ssl.create_default_context(cafile=CA_FILE)
            # The existing private homelab CA predates Python/OpenSSL strict
            # X.509 extension enforcement. Keep chain and hostname checks,
            # but allow that legacy CA until it is rotated.
            ctx.verify_flags &= ~ssl.VERIFY_X509_STRICT
            req = urllib.request.Request(SERVER + "/api/v1/auth/badge", data=payload, headers={"Content-Type": "application/json"})
            with urllib.request.urlopen(req, context=ctx, timeout=12) as response:
                result = json.load(response)
            GLib.idle_add(self._apply_auth, result)
        except Exception:
            GLib.idle_add(self.show_state, "Server nicht erreichbar", "Verbindung zum Loginserver fehlgeschlagen.", "×", "", False, True)

    def _apply_auth(self, result):
        if result.get("valid") and result.get("login_grant"):
            username = result.get("username", "")
            self.show_state("Willkommen, " + result.get("display_name", username), "Sitzung wird geöffnet …", "✓", self.badge_id)
            GLib.timeout_add(350, self._badge_login, username, result["login_grant"])
        elif result.get("reason") in ("pin_required", "invalid_pin"):
            message = "Persönliche PIN eingeben" if result.get("reason") == "pin_required" else "PIN war falsch – erneut versuchen"
            self.show_state("PIN erforderlich", message, "▣", self.badge_id, pin=True)
        else:
            self.show_state("Zugriff verweigert", "Der Badge ist nicht freigegeben.", "×", self.badge_id, retry=True)
        return False

    def _badge_login(self, username, grant):
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,63}", username):
            self.show_state("Anmeldung fehlgeschlagen", "Ungültiger AD-Benutzer.", "×", retry=True)
            return False
        tmp = GRANT_FILE + "." + str(os.getpid())
        fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        try:
            os.write(fd, (username + "\n" + grant + "\n").encode())
        finally:
            os.close(fd)
        os.replace(tmp, GRANT_FILE)
        self.auth_mode = "badge"
        self.greeter.authenticate(username)
        return False

    def password_fallback(self, _button):
        self.mode = "password"
        self.pin_box.hide()
        self.retry.hide()
        self.fallback.hide()
        self.title_label.set_text("AD-Anmeldung")
        self.message.set_text("Mit Benutzername und Passwort anmelden")
        self.icon.set_text("●")
        self.badge.set_text("")
        self.login_box.show_all()
        self.password.hide()
        self.username.set_sensitive(True)
        self.username.grab_focus()

    def badge_mode(self, _button=None):
        if self.greeter.get_in_authentication():
            self.greeter.cancel_authentication()
        self.mode = "badge"
        self.auth_mode = ""
        self.login_box.hide()
        self.fallback.show()
        self.start_scan()

    def password_action(self, _button):
        if self.password.get_visible():
            self.submit_password(self.password)
        else:
            self.begin_password_login(self.username)

    def begin_password_login(self, _widget):
        username = self.username.get_text().strip()
        if not username:
            self.message.set_text("Bitte AD-Benutzernamen eingeben")
            return
        self.auth_mode = "password"
        self.username.set_sensitive(False)
        self.message.set_text("Anmeldung wird vorbereitet …")
        self.greeter.authenticate(username)

    def submit_password(self, _widget):
        value = self.password.get_text()
        self.password.set_text("")
        self.message.set_text("Anmeldedaten werden geprüft …")
        self.greeter.respond(value)
        value = ""

    def on_show_prompt(self, _greeter, text, prompt_type):
        self.mode = "password"
        self.login_box.show_all()
        self.fallback.hide()
        self.message.set_text(text or "AD-Passwort eingeben")
        self.password.set_visibility(prompt_type != LightDM.PromptType.SECRET)
        self.password.show()
        self.password.grab_focus()

    def on_show_message(self, _greeter, text, _message_type):
        if text:
            self.message.set_text(text)

    def on_authentication_complete(self, greeter):
        if greeter.get_is_authenticated():
            self.show_state("Willkommen", "Desktop wird gestartet …", "✓")
            session = self.sessions.get_active_id() or greeter.get_default_session_hint() or "xfce"
            try:
                if not greeter.start_session_sync(session):
                    raise RuntimeError("LightDM hat den Sitzungsstart abgelehnt")
            except Exception as error:
                self.show_state("Sitzungsstart fehlgeschlagen", str(error), "×", retry=True)
        elif self.auth_mode == "password":
            self.password_fallback(None)
            self.message.set_text("Benutzername oder Passwort ist falsch")
        else:
            self.show_state("Badge-Anmeldung fehlgeschlagen", "Bitte erneut scannen oder AD-Passwort verwenden.", "×", retry=True)

    def power_action(self, action):
        try:
            if action == "restart" and LightDM.get_can_restart():
                LightDM.restart()
            elif action == "shutdown" and LightDM.get_can_shutdown():
                LightDM.shutdown()
            else:
                self.message.set_text("Diese Aktion ist momentan nicht verfügbar")
        except Exception as error:
            self.message.set_text(str(error))


if __name__ == "__main__":
    window = NativeGreeter()
    window.show_all()
    window.pin_box.hide()
    window.retry.hide()
    window.login_box.hide()
    screen = Gdk.Screen.get_default()
    window.resize(screen.get_width(), screen.get_height())
    Gtk.main()
