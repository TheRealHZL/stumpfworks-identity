import AVFoundation
import CoreMedia
import Foundation
import Vision

final class QRScanner: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {
    private let session = AVCaptureSession()
    private let queue = DispatchQueue(label: "de.stumpfworks.badge-camera.frames")
    private let lock = NSLock()
    private var finished = false
    private var processing = false

    func start() throws {
        guard let camera = AVCaptureDevice.default(for: .video) else {
            throw NSError(domain: "SWBadgeCamera", code: 1, userInfo: [NSLocalizedDescriptionKey: "No camera found"])
        }
        let input = try AVCaptureDeviceInput(device: camera)
        guard session.canAddInput(input) else { throw NSError(domain: "SWBadgeCamera", code: 2, userInfo: [NSLocalizedDescriptionKey: "Cannot use camera input"]) }
        session.addInput(input)
        let output = AVCaptureVideoDataOutput()
        output.alwaysDiscardsLateVideoFrames = true
        output.setSampleBufferDelegate(self, queue: queue)
        guard session.canAddOutput(output) else { throw NSError(domain: "SWBadgeCamera", code: 3, userInfo: [NSLocalizedDescriptionKey: "Cannot create camera video output"]) }
        session.addOutput(output)
        fputs("Camera ready. Hold the StumpfWorks QR badge in front of the camera...\n", stderr)
        session.startRunning()
        while !isFinished() { RunLoop.current.run(mode: .default, before: Date(timeIntervalSinceNow: 0.1)) }
        session.stopRunning()
    }

    func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
        lock.lock()
        if finished || processing { lock.unlock(); return }
        processing = true
        lock.unlock()
        defer { lock.lock(); processing = false; lock.unlock() }
        guard let buffer = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
        let request = VNDetectBarcodesRequest()
        request.symbologies = [.qr]
        do {
            try VNImageRequestHandler(cvPixelBuffer: buffer, orientation: .up, options: [:]).perform([request])
            guard let observations = request.results,
                  let payload = observations.compactMap({ $0.payloadStringValue }).first(where: { $0.hasPrefix("SWBADGE:1:") }) else { return }
            lock.lock()
            if !finished { finished = true; print(payload); fflush(stdout) }
            lock.unlock()
        } catch {
            fputs("Vision scan warning: \(error.localizedDescription)\n", stderr)
        }
    }

    private func isFinished() -> Bool { lock.lock(); defer { lock.unlock() }; return finished }
}

func runScanner() {
    do { try QRScanner().start(); exit(EXIT_SUCCESS) }
    catch { fputs("Camera error: \(error.localizedDescription)\n", stderr); exit(EXIT_FAILURE) }
}

switch AVCaptureDevice.authorizationStatus(for: .video) {
case .authorized: runScanner()
case .notDetermined:
    AVCaptureDevice.requestAccess(for: .video) { granted in
        if granted { runScanner() }
        else { fputs("Camera permission was denied. Enable it in System Settings > Privacy & Security > Camera.\n", stderr); exit(EXIT_FAILURE) }
    }
    dispatchMain()
default:
    fputs("Camera permission is unavailable. Enable it in System Settings > Privacy & Security > Camera.\n", stderr)
    exit(EXIT_FAILURE)
}
