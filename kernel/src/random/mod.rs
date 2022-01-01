//! random provides a cryptographically secure pseudo-random number generator (CSPRNG).

mod csprng;

use crate::multitasking::thread::Thread;
use crate::time;
use crate::time::Duration;
use alloc::boxed::Box;
use alloc::vec::Vec;

/// CSPRNG is the kernel's cryptographically secure pseudo-random number generator.
///
/// CSPRNG must be seeded by at least one source of entropy before use. The kernel
/// will panic if read is called before CSPRNG has been seeded. We also start a
/// companion kernel thread to add more entropy from each available source every
/// 30 seconds.
///
/// We do not currently expose add_entropy more widely, but that may change in the
/// future.
///
static CSPRNG: spin::Mutex<csprng::Csprng> = spin::Mutex::new(csprng::Csprng::new());

/// read fills the given buffer with random data.
///
/// read will panic if the CSPRNG has not been seeded by registering at least one
/// entropy source, then calling init.
///
pub fn read(buf: &mut [u8]) {
    CSPRNG.lock().read(buf);
}

/// EntropySource is a trait we use to simplify the process of collecting sources
/// of entropy.
///
pub trait EntropySource: Send {
    /// get_entropy fills the given buffer with entropy.
    ///
    fn get_entropy(&mut self, buf: &mut [u8; 32]);
}

/// ENTROPY_SOURCES is our set of entropy sources, supplied using register_entropy_source.
///
static ENTROPY_SOURCES: spin::Mutex<Vec<Box<dyn EntropySource>>> = spin::Mutex::new(Vec::new());

/// register_entropy_source is used to provide an ongoing source of entropy to the
/// kernel for use in seeding the CSPRNG.
///
pub fn register_entropy_source(src: Box<dyn EntropySource>) {
    ENTROPY_SOURCES.lock().push(src);
}

/// init initialises the CSPNRG using the entropy sources that have been registered,
/// then starts the companion thread to ensure the CSPRNG gets a steady feed of entropy
/// over time.
///
pub fn init() {
    let mut csprng = CSPRNG.lock();
    let mut sources = ENTROPY_SOURCES.lock();
    if sources.is_empty() {
        panic!("random::init called without any entropy sources registered");
    }

    let mut buf = [0u8; 32];
    for source in sources.iter_mut() {
        source.get_entropy(&mut buf);
        csprng.seed(&buf);
    }

    Thread::start_kernel_thread(helper_entry_point);
}

/// HELPER_INTERVAL indicates for how long the companion thread sleeps between
/// each re-seeding of the CSPRNG.
///
const HELPER_INTERVAL: Duration = Duration::from_secs(30);

/// helper_entry_point is an entry point used by an
/// entropy management thread to ensure the CSPRNG
/// continues to receive entropy over time.
///
fn helper_entry_point() -> ! {
    let mut buf = [0u8; 32];
    loop {
        time::sleep(HELPER_INTERVAL);

        crate::println!("Reseeding CSPRNG.");
        let mut csprng = CSPRNG.lock();
        let mut sources = ENTROPY_SOURCES.lock();
        if sources.is_empty() {
            panic!("all entropy sources removed");
        }

        for source in sources.iter_mut() {
            source.get_entropy(&mut buf);
            csprng.seed(&buf);
        }
    }
}
