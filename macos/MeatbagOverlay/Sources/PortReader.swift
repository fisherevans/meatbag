import Foundation

// Reads the port from ~/.meatbag/state.json written by the daemon.
// Falls back to 7421 (the default) if the file is absent or unreadable.
enum PortReader {
    static var port: Int {
        let stateURL = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".meatbag/state.json")
        guard
            let data = try? Data(contentsOf: stateURL),
            let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
            let p = json["port"] as? Int
        else { return 7421 }
        return p
    }
}
