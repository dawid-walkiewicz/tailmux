import Foundation

// Mirrors of the daemon's JSON (internal/mux/http.go, stats.go, exit.go).

struct StatusResponse: Decodable {
    var tailnets: [TailnetStatus]
    var tun: TUNInfo?
    var exitNode: ExitStatus?
    var version: String?
    var update: UpdateInfo?
}

struct UpdateInfo: Decodable {
    var current: String
    var latest: String?
    var available: Bool
    var url: String?
    var state: String
    var error: String?
    var auto: Bool
}

struct TailnetStatus: Decodable, Identifiable {
    var id: String { name }
    var name: String
    var enabled: Bool
    var state: String?
    var authUrl: String?
    var suffix: String?
    var selfIps: [String]?
    var peers: Int
    var online: Int
    var routes: [String]?
    var error: String?
    var lock: LockStatus?

    /// Tailnet Lock is on and nobody has signed this device yet: it's
    /// logged in, but the tailnet's devices ignore it.
    var needsSignature: Bool { lock.map { $0.enabled && !$0.signed } ?? false }
    var running: Bool { enabled && state == "Running" && !needsSignature }
}

struct LockStatus: Decodable {
    var enabled: Bool
    var signed: Bool
    var nodeKey: String?
    var publicKey: String?
    var signCommand: String?
    var trustedKeys: Int?
}

struct TUNInfo: Decodable {
    var interface: String
    var fakeRange: String
    var dns: Bool
}

struct StatsResponse: Decodable {
    var intervalMs: Int
    var tailnets: [TailnetStats]
}

struct TailnetStats: Decodable {
    var name: String
    var enabled: Bool
    var conns: Int
    var rxTotal: UInt64
    var txTotal: UInt64
    var rx: [Double]?
    var tx: [Double]?
}

struct PeerInfo: Decodable, Identifiable, Hashable {
    var id: String { tailnet + "/" + name }
    var tailnet: String
    var name: String
    var alias: String
    var fqdn: String
    var ips: [String]?
    var online: Bool
    var routes: [String]?
}

/// Where an exit node is. Set for Mullvad nodes; every field may be absent.
struct Location: Decodable, Hashable {
    var country: String?
    var countryCode: String?
    var city: String?
    var cityCode: String?
    var priority: Int?

    /// The country's flag, from its two-letter code.
    var flag: String {
        guard let cc = countryCode?.uppercased(), cc.count == 2 else { return "" }
        return String(String.UnicodeScalarView(cc.unicodeScalars.compactMap { Unicode.Scalar(127397 + $0.value) }))
    }
}

/// The chosen exit node and whether traffic can use it right now.
struct ExitStatus: Decodable {
    var tailnet: String
    var node: String
    var name: String?
    var fqdn: String?
    var online: Bool
    var active: Bool
    var location: Location?
    var error: String?

    var displayName: String { name.flatMap { $0.isEmpty ? nil : $0 } ?? node }
}

struct ExitNodeInfo: Decodable, Identifiable, Hashable {
    var id: String
    var tailnet: String
    var name: String
    var fqdn: String
    var ips: [String]?
    var online: Bool
    var mullvad: Bool?
    var location: Location?
    var selected: Bool?

    var isMullvad: Bool { mullvad ?? false }
    var isSelected: Bool { selected ?? false }
    var priority: Int { location?.priority ?? 0 }
    /// What PUT /exit-node gets as "node": the MagicDNS name, else the ID.
    var spec: String { fqdn.isEmpty ? id : fqdn }
    var key: String { tailnet + "/" + id }
}

struct ExitNodesResponse: Decodable {
    var current: ExitStatus?
    var nodes: [ExitNodeInfo]?
}

struct ConfigView: Decodable {
    var path: String
    var stateDir: String
    var config: SavedConfig
}

struct SavedConfig: Decodable {
    var hostname: String?
    var tailnets: [SavedTailnet]?
    var tun: SavedTUN?
    var updates: SavedUpdates?
}

struct SavedTailnet: Decodable {
    var name: String
    var controlUrl: String?
}

struct SavedTUN: Decodable { var enabled: Bool? }
struct SavedUpdates: Decodable { var check: Bool?; var auto: Bool? }

/// Talks to the daemon's HTTP API on loopback.
struct API {
    var base: URL

    /// Finds the daemon's API address from the same config files the CLI
    /// reads, falling back to the default port.
    static func discover() -> API {
        if let env = ProcessInfo.processInfo.environment["TAILMUX_API"], let url = URL(string: env) {
            return API(base: url)
        }
        for p in configPaths() {
            guard let data = FileManager.default.contents(atPath: p),
                  let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
            else { continue }
            var addr = (obj["http"] as? String) ?? "127.0.0.1:1056"
            if addr.hasPrefix(":") || addr.hasPrefix("0.0.0.0:") {
                addr = "127.0.0.1:" + (addr.split(separator: ":").last.map(String.init) ?? "1056")
            }
            if let url = URL(string: "http://" + addr) { return API(base: url) }
        }
        return API(base: URL(string: "http://127.0.0.1:1056")!)
    }

    private static func configPaths() -> [String] {
        var paths: [String] = []
        if let p = ProcessInfo.processInfo.environment["TAILMUX_CONFIG"] { paths.append(p) }
        paths.append(NSHomeDirectory() + "/.config/tailmux/config.json")
        paths.append("/opt/homebrew/etc/tailmux/config.json")
        paths.append("/usr/local/etc/tailmux/config.json")
        return paths
    }

    /// The daemon's API token: the api-token file next to the first config
    /// location that has one, readable only by the user who set tailmux
    /// up. Read for every request, so a new token is picked up.
    static func token() -> String {
        for p in configPaths() {
            let path = (p as NSString).deletingLastPathComponent + "/api-token"
            if let data = FileManager.default.contents(atPath: path),
               let s = String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines),
               !s.isEmpty {
                return s
            }
        }
        return ""
    }

    /// A request carrying the API token, which every endpoint requires.
    private func request(_ url: URL, _ method: String = "GET") -> URLRequest {
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue(API.token(), forHTTPHeaderField: "X-Tailmux")
        return req
    }

    private static let decoder: JSONDecoder = {
        let d = JSONDecoder()
        d.keyDecodingStrategy = .convertFromSnakeCase
        return d
    }()

    private static let session: URLSession = {
        let c = URLSessionConfiguration.ephemeral
        c.timeoutIntervalForRequest = 3
        c.connectionProxyDictionary = [:] // never route the API through a system proxy
        return URLSession(configuration: c)
    }()

    func get<T: Decodable>(_ path: String, as: T.Type) async throws -> T {
        let (data, resp) = try await API.session.data(for: request(base.appendingPathComponent(path)))
        try check(resp, data)
        return try API.decoder.decode(T.self, from: data)
    }

    func getLogs(_ n: Int) async throws -> [String] {
        var c = URLComponents(url: base.appendingPathComponent("logs"), resolvingAgainstBaseURL: false)!
        c.queryItems = [URLQueryItem(name: "n", value: String(n))]
        let (data, resp) = try await API.session.data(for: request(c.url!))
        try check(resp, data)
        return try API.decoder.decode([String].self, from: data)
    }

    /// Sends a changing request and returns the body.
    @discardableResult
    func send(_ method: String, _ path: String, json: [String: Any]? = nil) async throws -> Data {
        var req = request(base.appendingPathComponent(path), method)
        if let json {
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
            req.httpBody = try JSONSerialization.data(withJSONObject: json)
        }
        let (data, resp) = try await API.session.data(for: req)
        try check(resp, data)
        return data
    }

    func decode<T: Decodable>(_ data: Data, as: T.Type) throws -> T {
        try API.decoder.decode(T.self, from: data)
    }

    func setEnabled(_ name: String, _ on: Bool) async throws {
        let req = request(base.appendingPathComponent("tailnets/\(name)/\(on ? "enable" : "disable")"), "POST")
        let (data, resp) = try await API.session.data(for: req)
        try check(resp, data)
    }

    /// Picks an exit node, or turns it off with nil. Applies live.
    @discardableResult
    func setExitNode(tailnet: String?, node: String?) async throws -> ExitStatus? {
        var body: [String: Any] = [:]
        if let tailnet, let node {
            body = ["tailnet": tailnet, "node": node]
        }
        let data = try await send("PUT", "exit-node", json: body)
        return try decode(data, as: ExitNodesResponse.self).current
    }

    func startUpdate() async throws {
        let req = request(base.appendingPathComponent("update"), "POST")
        let (data, resp) = try await API.session.data(for: req)
        try check(resp, data)
    }

    private func check(_ resp: URLResponse, _ data: Data) throws {
        guard let http = resp as? HTTPURLResponse else { return }
        if !(200..<300).contains(http.statusCode) {
            let msg = String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            throw NSError(domain: "tailmux", code: http.statusCode, userInfo: [NSLocalizedDescriptionKey: msg])
        }
    }
}
