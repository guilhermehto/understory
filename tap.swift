// System-audio capture helper for the pomodoro visualizer (macOS 14.4+).
//
// Opens a global Core Audio process tap on the system output — no loopback
// driver needed — and streams to stdout:
//   1. a 4-byte little-endian uint32 sample-rate header (the output device's
//      native rate; the Go side must not assume 44.1kHz)
//   2. endless mono f32le PCM
// First run triggers the "System Audio Recording" permission prompt; errors go
// to stderr with exit 1. The parent kills us to stop capture; if it dies
// uncleanly, SIGPIPE on the next write does the job.

import CoreAudio
import Foundation

func fail(_ msg: String) -> Never {
    FileHandle.standardError.write(Data((msg + "\n").utf8))
    exit(1)
}

func check(_ status: OSStatus, _ what: String) {
    if status != noErr { fail("\(what) failed (OSStatus \(status))") }
}

@_silgen_name("responsibility_spawnattrs_setdisclaim")
func responsibility_spawnattrs_setdisclaim(_ attrs: UnsafeMutablePointer<posix_spawnattr_t?>, _ disclaim: Int32) -> Int32

// TCC attributes an unbundled CLI to its terminal, which won't carry
// NSAudioCaptureUsageDescription — tccd then refuses to prompt at all. Re-exec
// disclaimed (private API, the Chromium trick) so this binary, with its
// embedded __info_plist, is its own TCC subject and the prompt can fire.
if ProcessInfo.processInfo.environment["POMODORO_TAP_DISCLAIMED"] == nil {
    var attrs: posix_spawnattr_t?
    posix_spawnattr_init(&attrs)
    _ = responsibility_spawnattrs_setdisclaim(&attrs, 1)
    posix_spawnattr_setflags(&attrs, Int16(POSIX_SPAWN_SETEXEC))
    let path = Bundle.main.executablePath ?? CommandLine.arguments[0]
    var argv: [UnsafeMutablePointer<CChar>?] = [strdup(path), nil]
    var env: [UnsafeMutablePointer<CChar>?] = [strdup("POMODORO_TAP_DISCLAIMED=1"), nil]
    var pid: pid_t = 0
    // POSIX_SPAWN_SETEXEC replaces this image; returning means it failed.
    let rc = posix_spawn(&pid, path, nil, &attrs, &argv, &env)
    fail("disclaimed re-exec failed: \(String(cString: strerror(rc)))")
}

// Mono mixdown of everything routed to the default output.
let tapDesc = CATapDescription(monoGlobalTapButExcludeProcesses: [])
tapDesc.isPrivate = true
var tapID = AudioObjectID(kAudioObjectUnknown)
check(AudioHardwareCreateProcessTap(tapDesc, &tapID),
      "creating system-audio tap (needs macOS 14.4+ and System Audio Recording permission)")

// Wrap the tap in a private aggregate device so an IO proc can pull from it.
let aggDesc: [String: Any] = [
    kAudioAggregateDeviceNameKey: "pomodoro-tap",
    kAudioAggregateDeviceUIDKey: UUID().uuidString,
    kAudioAggregateDeviceIsPrivateKey: true,
    kAudioAggregateDeviceTapAutoStartKey: true,
    kAudioAggregateDeviceTapListKey: [
        [kAudioSubTapUIDKey: tapDesc.uuid.uuidString,
         kAudioSubTapDriftCompensationKey: true]
    ],
]
var aggID = AudioObjectID(kAudioObjectUnknown)
check(AudioHardwareCreateAggregateDevice(aggDesc as CFDictionary, &aggID),
      "creating aggregate device")

// Report the tap's true format so the analyzer bins land on real frequencies.
var fmt = AudioStreamBasicDescription()
var size = UInt32(MemoryLayout<AudioStreamBasicDescription>.size)
var addr = AudioObjectPropertyAddress(
    mSelector: kAudioTapPropertyFormat,
    mScope: kAudioObjectPropertyScopeGlobal,
    mElement: kAudioObjectPropertyElementMain)
check(AudioObjectGetPropertyData(tapID, &addr, 0, nil, &size, &fmt), "reading tap format")

var rate = UInt32(fmt.mSampleRate).littleEndian
withUnsafeBytes(of: &rate) { _ = write(1, $0.baseAddress, 4) }

let channels = max(1, Int(fmt.mChannelsPerFrame))
var mono = [Float32]()
var procID: AudioDeviceIOProcID?
// ponytail: blocking write(2) on the IO thread; harmless here — the aggregate
// is private and feeds only this pipe, worst case we drop to a broken parent.
check(AudioDeviceCreateIOProcIDWithBlock(&procID, aggID, nil) { _, inData, _, _, _ in
    let bufs = UnsafeMutableAudioBufferListPointer(UnsafeMutablePointer(mutating: inData))
    guard let buf = bufs.first, let ptr = buf.mData else { return }
    let n = Int(buf.mDataByteSize) / 4
    let samples = ptr.bindMemory(to: Float32.self, capacity: n)
    if channels == 1 {
        _ = write(1, samples, n * 4)
        return
    }
    let frames = n / channels
    if mono.count < frames { mono = [Float32](repeating: 0, count: frames) }
    for f in 0..<frames {
        var sum: Float32 = 0
        for c in 0..<channels { sum += samples[f * channels + c] }
        mono[f] = sum / Float32(channels)
    }
    mono.withUnsafeBytes { _ = write(1, $0.baseAddress, frames * 4) }
}, "installing IO proc")
check(AudioDeviceStart(aggID, procID), "starting capture")

dispatchMain()
