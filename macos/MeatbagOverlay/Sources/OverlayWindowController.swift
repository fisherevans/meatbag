import AppKit
import WebKit

// Borderless NSPanel returns false for canBecomeKey by default, which prevents
// the WKWebView from receiving keyboard events. Override to allow text input.
private class OverlayPanel: NSPanel {
    override var canBecomeKey: Bool { true }

    // Non-activating panels skip the app menu bar, so Cmd+C/V/X/A/Z never
    // reach the WKWebView. Forward them directly to the first responder.
    override func performKeyEquivalent(with event: NSEvent) -> Bool {
        guard event.modifierFlags.contains(.command) else {
            return super.performKeyEquivalent(with: event)
        }
        switch event.charactersIgnoringModifiers {
        case "c": return NSApp.sendAction(#selector(NSText.copy(_:)),      to: nil, from: self)
        case "v": return NSApp.sendAction(#selector(NSText.paste(_:)),     to: nil, from: self)
        case "x": return NSApp.sendAction(#selector(NSText.cut(_:)),       to: nil, from: self)
        case "a": return NSApp.sendAction(#selector(NSText.selectAll(_:)), to: nil, from: self)
        case "z": return NSApp.sendAction(Selector(("undo:")),             to: nil, from: self)
        default:  return super.performKeyEquivalent(with: event)
        }
    }
}

// Tracks mouse enter/exit on the window surface to fade opacity in/out.
private class TrackingContentView: NSView {
    var onMouseEnter: (() -> Void)?
    var onMouseExit: (() -> Void)?

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        trackingAreas.forEach { removeTrackingArea($0) }
        addTrackingArea(NSTrackingArea(
            rect: bounds,
            options: [.mouseEnteredAndExited, .activeAlways, .inVisibleRect],
            owner: self,
            userInfo: nil
        ))
    }

    override func mouseEntered(with event: NSEvent) { onMouseEnter?() }
    override func mouseExited(with event: NSEvent) { onMouseExit?() }
}

class OverlayWindowController: NSWindowController {
    private var webView: WKWebView!
    private var collapseButton: NSButton!
    private var opacitySlider: NSSlider!
    private var isCollapsed = false
    private var expandedHeight: CGFloat = 480
    private var bgOpacity: CGFloat = 0.5

    static let kInternetEventClass: AEEventClass = 0x4755524C
    static let kAEGetURL: AEEventID            = 0x4755524C
    static let keyDirectObject: AEKeyword      = 0x2D2D2D2D

    convenience init() {
        let panel = OverlayWindowController.makePanel()
        self.init(window: panel)
        buildContent(in: panel)
        loadPlaceholder()
    }

    // MARK: - Window setup

    private static func makePanel() -> NSPanel {
        let panel = OverlayPanel(
            contentRect: NSRect(x: 0, y: 0, width: 400, height: 480),
            styleMask: [.borderless, .resizable, .nonactivatingPanel],
            backing: .buffered,
            defer: false
        )
        panel.level = .floating
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]
        panel.isMovableByWindowBackground = true
        panel.backgroundColor = .clear
        panel.isOpaque = false
        panel.hasShadow = true
        panel.alphaValue = 1.0
        panel.center()
        return panel
    }

    private func buildContent(in panel: NSPanel) {
        let root = TrackingContentView()
        root.wantsLayer = true
        root.layer?.cornerRadius = 10
        root.layer?.masksToBounds = true
        root.layer?.backgroundColor = NSColor(
            calibratedRed: 0.07, green: 0.08, blue: 0.10, alpha: 0.97
        ).cgColor
        panel.contentView = root

        root.onMouseEnter = { [weak self] in
            NSAnimationContext.runAnimationGroup { ctx in
                ctx.duration = 0.15
                self?.window?.animator().alphaValue = 1.0
            }
        }
        root.onMouseExit = { [weak self] in
            guard let self else { return }
            NSAnimationContext.runAnimationGroup { ctx in
                ctx.duration = 0.3
                self.window?.animator().alphaValue = self.bgOpacity
            }
        }

        buildControlBar(in: root)
        buildWebView(in: root)

        panel.minSize = NSSize(width: 280, height: 200)
        panel.maxSize = NSSize(width: 900, height: 1200)
    }

    private func buildControlBar(in parent: NSView) {
        let bar = NSView()
        bar.wantsLayer = true
        bar.layer?.backgroundColor = NSColor(
            calibratedRed: 0.10, green: 0.12, blue: 0.16, alpha: 1
        ).cgColor
        // Bottom border separating bar from webview
        let border = NSView()
        border.wantsLayer = true
        border.layer?.backgroundColor = NSColor(white: 1, alpha: 0.08).cgColor
        border.translatesAutoresizingMaskIntoConstraints = false
        bar.addSubview(border)
        bar.translatesAutoresizingMaskIntoConstraints = false
        parent.addSubview(bar)

        NSLayoutConstraint.activate([
            bar.topAnchor.constraint(equalTo: parent.topAnchor),
            bar.leadingAnchor.constraint(equalTo: parent.leadingAnchor),
            bar.trailingAnchor.constraint(equalTo: parent.trailingAnchor),
            bar.heightAnchor.constraint(equalToConstant: 34),
            border.bottomAnchor.constraint(equalTo: bar.bottomAnchor),
            border.leadingAnchor.constraint(equalTo: bar.leadingAnchor),
            border.trailingAnchor.constraint(equalTo: bar.trailingAnchor),
            border.heightAnchor.constraint(equalToConstant: 1),
        ])

        collapseButton = makeBarButton(title: "−", action: #selector(toggleCollapse), width: 26)
        bar.addSubview(collapseButton)

        let closeButton = makeBarButton(title: "✕", action: #selector(closeOverlay), width: 26)
        bar.addSubview(closeButton)

        let opacityLabel = NSTextField(labelWithString: "opacity")
        opacityLabel.font = NSFont.systemFont(ofSize: 10)
        opacityLabel.textColor = NSColor(white: 1, alpha: 0.35)
        opacityLabel.translatesAutoresizingMaskIntoConstraints = false
        bar.addSubview(opacityLabel)

        opacitySlider = NSSlider(value: Double(bgOpacity), minValue: 0.05, maxValue: 0.95,
                                 target: self, action: #selector(opacityChanged))
        opacitySlider.controlSize = .small
        opacitySlider.translatesAutoresizingMaskIntoConstraints = false
        bar.addSubview(opacitySlider)

        NSLayoutConstraint.activate([
            collapseButton.leadingAnchor.constraint(equalTo: bar.leadingAnchor, constant: 6),
            collapseButton.centerYAnchor.constraint(equalTo: bar.centerYAnchor, constant: -1),

            closeButton.trailingAnchor.constraint(equalTo: bar.trailingAnchor, constant: -6),
            closeButton.centerYAnchor.constraint(equalTo: bar.centerYAnchor, constant: -1),

            opacitySlider.trailingAnchor.constraint(equalTo: closeButton.leadingAnchor, constant: -8),
            opacitySlider.centerYAnchor.constraint(equalTo: bar.centerYAnchor, constant: -1),
            opacitySlider.widthAnchor.constraint(equalToConstant: 80),

            opacityLabel.trailingAnchor.constraint(equalTo: opacitySlider.leadingAnchor, constant: -5),
            opacityLabel.centerYAnchor.constraint(equalTo: bar.centerYAnchor, constant: -1),
        ])
    }

    private func makeBarButton(title: String, action: Selector, width: CGFloat = 24) -> NSButton {
        let b = NSButton(frame: .zero)
        b.title = ""
        b.attributedTitle = NSAttributedString(string: title, attributes: [
            .foregroundColor: NSColor(white: 1, alpha: 0.55),
            .font: NSFont.systemFont(ofSize: 13, weight: .regular),
        ])
        b.bezelStyle = .inline
        b.isBordered = false
        b.target = self
        b.action = action
        b.translatesAutoresizingMaskIntoConstraints = false
        b.widthAnchor.constraint(equalToConstant: width).isActive = true
        return b
    }

    private func setCollapseButtonTitle(_ title: String) {
        collapseButton.attributedTitle = NSAttributedString(string: title, attributes: [
            .foregroundColor: NSColor(white: 1, alpha: 0.55),
            .font: NSFont.systemFont(ofSize: 13, weight: .regular),
        ])
    }

    private func buildWebView(in parent: NSView) {
        let config = WKWebViewConfiguration()
        webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = self
        webView.translatesAutoresizingMaskIntoConstraints = false
        webView.pageZoom = 0.85
        if #available(macOS 12.0, *) {
            webView.underPageBackgroundColor = NSColor(
                calibratedRed: 0.07, green: 0.08, blue: 0.10, alpha: 1
            )
        }
        parent.addSubview(webView)

        NSLayoutConstraint.activate([
            webView.topAnchor.constraint(equalTo: parent.topAnchor, constant: 34),
            webView.leadingAnchor.constraint(equalTo: parent.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: parent.trailingAnchor),
            webView.bottomAnchor.constraint(equalTo: parent.bottomAnchor),
        ])
    }

    // MARK: - Navigation

    // Show a plain placeholder until navigate() is called via URL scheme.
    private func loadPlaceholder() {
        let html = """
        <!doctype html>
        <html>
        <head><meta charset=utf-8></head>
        <body style="margin:0;background:#0c0e13;color:#5d6678;
                     font:13px -apple-system,sans-serif;
                     display:flex;align-items:center;justify-content:center;
                     height:100vh;text-align:center">
          <p>Click &ldquo;Pop out&rdquo; in a meatbag list<br>to open a task here.</p>
        </body>
        </html>
        """
        webView.loadHTMLString(html, baseURL: nil)
    }

    func navigate(to slug: String, item itemID: String?) {
        let port = PortReader.port
        var comps = URLComponents()
        comps.scheme = "http"
        comps.host = "localhost"
        comps.port = port
        comps.path = "/popout/\(slug)"
        if let id = itemID, !id.isEmpty {
            comps.queryItems = [URLQueryItem(name: "item", value: id)]
        }
        guard let url = comps.url else { return }
        webView.load(URLRequest(url: url))
        if isCollapsed { toggleCollapse() }
    }

    // MARK: - Control actions

    @objc private func toggleCollapse() {
        guard let win = window else { return }
        if isCollapsed {
            isCollapsed = false
            setCollapseButtonTitle("−")
            webView.isHidden = false
            var frame = win.frame
            frame.origin.y -= (expandedHeight - 34)
            frame.size.height = expandedHeight
            win.setFrame(frame, display: true, animate: true)
            win.minSize = NSSize(width: 280, height: 200)
            win.maxSize = NSSize(width: 900, height: 1200)
        } else {
            isCollapsed = true
            expandedHeight = win.frame.height
            setCollapseButtonTitle("+")
            webView.isHidden = true
            var frame = win.frame
            frame.origin.y += (frame.size.height - 34)
            frame.size.height = 34
            win.setFrame(frame, display: true, animate: true)
            win.minSize = NSSize(width: 280, height: 34)
            win.maxSize = NSSize(width: 900, height: 34)
        }
    }

    @objc private func opacityChanged(_ sender: NSSlider) {
        bgOpacity = CGFloat(sender.doubleValue)
        guard let win = window else { return }
        if !win.frame.contains(NSEvent.mouseLocation) {
            win.alphaValue = bgOpacity
        }
    }

    @objc private func closeOverlay() {
        window?.orderOut(nil)
    }

    override func showWindow(_ sender: Any?) {
        super.showWindow(sender)
        window?.orderFrontRegardless()
    }
}

// MARK: - WKNavigationDelegate

extension OverlayWindowController: WKNavigationDelegate {
    func webView(
        _ webView: WKWebView,
        decidePolicyFor navigationAction: WKNavigationAction,
        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
    ) {
        guard
            navigationAction.navigationType == .linkActivated,
            let url = navigationAction.request.url
        else {
            decisionHandler(.allow)
            return
        }

        // Internal traffic: localhost (meatbag daemon) and scheme-less blobs/data.
        let isLocal = url.host == "localhost"
            || url.scheme == "data"
            || url.scheme == "blob"
            || url.scheme == "about"
        if isLocal {
            decisionHandler(.allow)
            return
        }

        // Everything else opens in the user's default browser.
        NSWorkspace.shared.open(url)
        decisionHandler(.cancel)
    }
}
