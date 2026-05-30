import AppKit

class AppDelegate: NSObject, NSApplicationDelegate {
    var windowController: OverlayWindowController?
    var statusItem: NSStatusItem?

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Register for meatbag:// URL scheme callbacks via Apple Events.
        NSAppleEventManager.shared().setEventHandler(
            self,
            andSelector: #selector(handleGetURLEvent(_:withReplyEvent:)),
            forEventClass: OverlayWindowController.kInternetEventClass,
            andEventID: OverlayWindowController.kAEGetURL
        )

        setupStatusBar()

        windowController = OverlayWindowController()
        windowController?.showWindow(nil)
    }

    // MARK: - Status bar

    private func setupStatusBar() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        if let button = statusItem?.button {
            button.title = "M"
            button.font = NSFont.monospacedSystemFont(ofSize: 11, weight: .bold)
            button.toolTip = "Meatbag Overlay"
        }

        let menu = NSMenu()
        menu.addItem(NSMenuItem(title: "Show Overlay", action: #selector(showOverlay), keyEquivalent: ""))
        menu.addItem(.separator())
        menu.addItem(NSMenuItem(title: "Quit", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"))
        statusItem?.menu = menu
    }

    @objc private func showOverlay() {
        windowController?.showWindow(nil)
        windowController?.window?.orderFrontRegardless()
    }

    // MARK: - URL scheme handler

    @objc func handleGetURLEvent(
        _ event: NSAppleEventDescriptor,
        withReplyEvent _: NSAppleEventDescriptor
    ) {
        guard
            let urlString = event
                .paramDescriptor(forKeyword: OverlayWindowController.keyDirectObject)?
                .stringValue,
            let url = URL(string: urlString)
        else { return }
        handleOpen(url: url)
    }

    private func handleOpen(url: URL) {
        guard
            url.scheme == "meatbag",
            url.host == "open",
            let comps = URLComponents(url: url, resolvingAgainstBaseURL: false)
        else { return }

        var params: [String: String] = [:]
        for item in comps.queryItems ?? [] {
            if let v = item.value { params[item.name] = v }
        }

        guard let list = params["list"] else { return }
        let itemID = params["item"]

        windowController?.navigate(to: list, item: itemID)
        windowController?.window?.orderFrontRegardless()
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows _: Bool) -> Bool {
        windowController?.showWindow(nil)
        return true
    }
}
