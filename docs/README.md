# Springfield CSI Driver Documentation

## Quick Navigation

### For Users
- **[Main README](../README.md)** - Installation and usage
- **[MIGRATION.md](MIGRATION.md)** - Upgrading from Python v0.1 to Go v0.2

### For Developers

#### Architecture Decision
- **[RECOMMENDED_ARCHITECTURE.md](RECOMMENDED_ARCHITECTURE.md)** ⭐ **START HERE**
  - Production-ready architecture without Blivet DBus
  - Complete implementation guide
  - Deployment examples

- **[ARCHITECTURE_COMPARISON.md](ARCHITECTURE_COMPARISON.md)**
  - Compares 4 different approaches
  - Pros/cons of each design
  - Why direct execution is best

#### Implementation
- **[REFACTORING_GUIDE.md](REFACTORING_GUIDE.md)**
  - Step-by-step migration from Blivet to direct execution
  - Code examples (before/after)
  - Detailed checklist

- **[Example: LVM Manager](../pkg/storage/lvm.go)**
  - Working implementation of direct LVM management
  - Shows nsenter pattern
  - Production-ready code

## The Question

> "Is there a better way to design a CSI driver for local storage?"

**TL;DR: Yes! Use direct command execution with nsenter instead of Blivet DBus.**

## The Problem with Current Design

```
Current (v0.2):  CSI Driver → DBus → Blivet Server → lvm/mdadm/etc
                              ❌ Buggy, complex, stateful

Recommended:     CSI Driver → nsenter → lvm/mdadm/etc
                              ✅ Simple, reliable, stateless
```

## Quick Comparison

| Aspect | Blivet DBus (Current) | Direct Execution (Recommended) |
|--------|----------------------|--------------------------------|
| **Complexity** | High | Low |
| **Dependencies** | Python, DBus, Blivet | Just storage CLIs |
| **Reliability** | Low (buggy) | High (battle-tested) |
| **State** | Stateful (sync issues) | Stateless |
| **Performance** | Medium | Fast |
| **Debugging** | Hard | Easy |
| **Production Use** | No | Yes (OpenEBS, TopoLVM) |

## Recommended Next Steps

1. **Read [RECOMMENDED_ARCHITECTURE.md](RECOMMENDED_ARCHITECTURE.md)**
   - Understand the proposed design
   - See complete code examples
   - Review deployment configuration

2. **Review [Example LVM Manager](../pkg/storage/lvm.go)**
   - Working implementation already provided
   - Shows nsenter pattern
   - Ready to integrate

3. **Follow [REFACTORING_GUIDE.md](REFACTORING_GUIDE.md)**
   - Migrate controller step-by-step
   - Update deployments
   - Test thoroughly

4. **Simplify!**
   - Remove `pkg/blivet/` package
   - No more Blivet DBus server
   - Simpler, more reliable driver

## Benefits

✅ **No external daemon** - One less thing to manage and debug  
✅ **Industry standard** - Same pattern as OpenEBS and TopoLVM  
✅ **Easier to debug** - See exact commands in logs  
✅ **More reliable** - No DBus state to desync  
✅ **Better performance** - No marshalling overhead  
✅ **Production ready** - Battle-tested approach  

## Estimated Effort

- **Reading docs**: 1-2 hours
- **LVM manager**: Already implemented! ✅
- **Controller refactor**: 2-3 hours
- **Testing**: 2-4 hours
- **Other storage types**: 1-2 hours each (optional)

**Total: 1-2 days of focused work**

## Alternative Approaches Considered

### 1. Directory-based LocalPV (Simplest)
- Just use directories, no LVM/RAID
- Perfect for development/testing
- Used by Rancher local-path-provisioner

### 2. Dedicated Storage Daemon
- Separate daemon on host (not Blivet)
- More complex but cleaner separation
- Used by TopoLVM's `lvmd`

### 3. Direct Execution (Recommended) ⭐
- Execute storage commands via nsenter
- No daemon, no DBus, no complexity
- Used by OpenEBS LocalPV

### 4. Blivet DBus (Current)
- Complex, buggy, not recommended
- Only benefit: abstraction over storage types
- Not worth the complexity cost

See [ARCHITECTURE_COMPARISON.md](ARCHITECTURE_COMPARISON.md) for detailed analysis.

## Code Examples

### Current (Blivet DBus)
```go
// Complex, requires external daemon
blivetClient, err := blivet.NewClient()
objPath, blockPath, err := blivetClient.CreateVolume(
    name, devicePaths, storageType, size)
```

### Recommended (Direct Execution)
```go
// Simple, direct, no daemon
lvmMgr := storage.NewLVMManager(executor)
devicePath, err := lvmMgr.CreateLogicalVolume(
    "csi-vg", name, size)
err = lvmMgr.CreateFilesystem(devicePath, "xfs")
```

Much cleaner! 🎉

## Questions?

- Review the architecture docs in this directory
- Check the example LVM manager implementation
- The refactoring guide has step-by-step instructions

## Contributing

If you improve the architecture or find issues:

1. Update relevant docs in this directory
2. Update example code in `pkg/storage/`
3. Submit a PR with clear description

## License

GNU General Public License v.2 or later (same as main project)
