// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements functionality to help parse the header of a file in a
//! [TAR](https://en.wikipedia.org/wiki/Tar_(computing)) archives.

/// Trims any leading or trailing spaces and null bytes, returning
/// the resulting (possibly empty) slice.
///
#[inline(always)]
fn trim_spaces_and_nulls(buf: &[u8]) -> &[u8] {
    let n = buf.len();
    if n == 0 {
        return buf;
    }

    let mut start = 0;
    while start < n {
        if buf[start] != 0 && buf[start] != b' ' {
            break;
        }

        start += 1;
    }

    let mut end = n - 1;
    while end > start {
        if buf[end] != 0 && buf[end] != b' ' {
            break;
        }

        end -= 1;
    }

    &buf[start..end + 1]
}

/// Parse an integer from the buffer in base 256.
///
/// Here, all bits after the first are a two's complement number
/// in big-endian order.
///
#[inline(always)]
fn parse_base256(buf: &[u8]) -> Option<isize> {
    // To handle negative values, we use the fact that:
    //    -a - 1 == ^a
    //
    // If the number is negative, we invert its bytes,
    // therefore processing an unsigned value. We then
    // negate it just before returning.
    let bitflip = if buf[0] & 0x40 != 0 {
        0xff // Flip bits to process as unsigned.
    } else {
        0x00 // Leave alone if zero or positive.
    };

    let mut val = 0usize;
    for (i, digit) in buf.iter().enumerate() {
        // Invert the digit if negative.
        let mut digit = *digit ^ bitflip;

        if i == 0 {
            // Ignore the first bit, which indicates
            // a base-256 number.
            digit &= 0x7f;
        }

        if val >> 56 > 0 {
            // Integer overflow.
            return None;
        }

        val <<= 8;
        val += digit as usize;
    }

    if val >> 63 > 0 {
        // Integer overflow.
        return None;
    }

    if bitflip == 0xff {
        Some(!(val as isize))
    } else {
        Some(val as isize)
    }
}

/// Parse an integer from the buffer in base 8 (octal).
///
#[inline(always)]
fn parse_octal(buf: &[u8]) -> Option<isize> {
    // Trim off any leading or trailing spaces and null bytes.
    let buf = trim_spaces_and_nulls(buf);
    if buf.is_empty() {
        return Some(0);
    }

    let mut val = 0;
    let mut neg = false;
    for (i, digit) in buf.iter().enumerate() {
        if i == 0 && buf.len() > 1 && (*digit == b'-' || *digit == b'+') {
            neg = *digit == b'-';
            continue;
        }

        // We stop at a null byte.
        if *digit == 0 {
            break;
        }

        if *digit < b'0' || b'7' < *digit {
            // Invalid character.
            return None;
        }

        val <<= 3; // *= 8.
        val += (*digit - b'0') as isize;
        continue;
    }

    if neg {
        Some(-val)
    } else {
        Some(val)
    }
}

/// Parse an integer from the buffer in either octal (base 8) or
/// base 256. The buffer must end in either a space or a null
/// byte.
///
/// If the number is not encoded correctly, `parse_number` returns
/// `None`.
///
pub fn parse_number(buf: &[u8]) -> Option<isize> {
    if !buf.is_empty() && buf[0] & 0x80 != 0 {
        parse_base256(buf)
    } else {
        parse_octal(buf)
    }
}

/// Parse an ASCII string from the buffer. This removes any null
/// terminator.
///
pub fn parse_string(buf: &[u8]) -> &[u8] {
    for (i, b) in buf.iter().enumerate() {
        if *b == 0 {
            return &buf[..i];
        }
    }

    buf
}

/// Parse the checksum value from a header.
///
/// POSIX specifies a sum of the unsigned byte values, but the
/// Sun tar used signed byte values. We return both.
///
pub fn parse_checksum(buf: &[u8]) -> (isize, isize) {
    let mut signed = 0;
    let mut unsigned = 0;
    for (i, b) in buf.iter().enumerate() {
        let b = if (148..(148 + 8)).contains(&i) {
            b' ' // Treat the checksum field as spaces.
        } else {
            *b
        };

        unsigned += b as isize;
        signed += b as i8 as isize;
    }

    (signed, unsigned)
}

#[test]
fn test_trim_spaces_and_nulls() {
    assert_eq!(trim_spaces_and_nulls(&[]), &[]);
    assert_eq!(trim_spaces_and_nulls(&[1u8]), &[1u8]);
    assert_eq!(trim_spaces_and_nulls(&[0u8, 0u8, 1u8, 0u8, 0u8]), &[1u8]);
    assert_eq!(
        trim_spaces_and_nulls(&[' ' as u8, 0u8, 1u8, 0u8, ' ' as u8]),
        &[1u8]
    );
    assert_eq!(
        trim_spaces_and_nulls(&[0u8, ' ' as u8, 1u8, ' ' as u8, 0u8]),
        &[1u8]
    );
    assert_eq!(
        trim_spaces_and_nulls(&[' ' as u8, ' ' as u8, 1u8, ' ' as u8, ' ' as u8]),
        &[1u8]
    );
}

#[test]
fn test_parse_octal() {
    assert_eq!(parse_number(b""), Some(0));
    assert_eq!(parse_number(b"777"), Some(0o777));
    assert_eq!(parse_number(&[0u8, 0u8, '3' as u8, 0u8, 0u8]), Some(3));

    // Additional test cases copied from Go's "archive/tar".
    assert_eq!(parse_number(b"0000000\x00"), Some(0));
    assert_eq!(parse_number(b" \x0000000\x00"), Some(0));
    assert_eq!(parse_number(b" \x0000003\x00"), Some(3));
    assert_eq!(parse_number(b"00000000227\x00"), Some(0o227));
    assert_eq!(parse_number(b"032033\x00 "), Some(0o32033));
    assert_eq!(parse_number(b"320330\x00 "), Some(0o320330));
    assert_eq!(parse_number(b"0000660\x00 "), Some(0o660));
    assert_eq!(parse_number(b"\x00 0000660\x00 "), Some(0o660));
    assert_eq!(parse_number(b"0123456789abcdef"), None);
    assert_eq!(parse_number(b"0123456789\x00abcdef"), None);
    assert_eq!(parse_number(b"01234567\x0089abcdef"), Some(342391));
    assert_eq!(parse_number(b"0123\x7e\x5f\x264123"), None);
}

#[test]
fn test_parse_base256() {
    // Test cases copied from Go's "archive/tar".
    assert_eq!(parse_number(b""), Some(0));
    assert_eq!(parse_number(b"\x80"), Some(0));
    assert_eq!(parse_number(b"\x80\x00"), Some(0));
    assert_eq!(parse_number(b"\x80\x00\x00"), Some(0));
    assert_eq!(parse_number(b"\xbf"), Some((1 << 6) - 1));
    assert_eq!(parse_number(b"\xbf\xff"), Some((1 << 14) - 1));
    assert_eq!(parse_number(b"\xbf\xff\xff"), Some((1 << 22) - 1));
    assert_eq!(parse_number(b"\xff"), Some(-1));
    assert_eq!(parse_number(b"\xff\xff"), Some(-1));
    assert_eq!(parse_number(b"\xff\xff\xff"), Some(-1));
    assert_eq!(parse_number(b"\xc0"), Some(-1 * (1 << 6)));
    assert_eq!(parse_number(b"\xc0\x00"), Some(-1 * (1 << 14)));
    assert_eq!(parse_number(b"\xc0\x00\x00"), Some(-1 * (1 << 22)));
    assert_eq!(
        parse_number(b"\x87\x76\xa2\x22\xeb\x8a\x72\x61"),
        Some(537795476381659745)
    );
    assert_eq!(
        parse_number(b"\x80\x00\x00\x00\x07\x76\xa2\x22\xeb\x8a\x72\x61"),
        Some(537795476381659745)
    );
    assert_eq!(
        parse_number(b"\xf7\x76\xa2\x22\xeb\x8a\x72\x61"),
        Some(-615126028225187231)
    );
    assert_eq!(
        parse_number(b"\xff\xff\xff\xff\xf7\x76\xa2\x22\xeb\x8a\x72\x61"),
        Some(-615126028225187231)
    );
    assert_eq!(
        parse_number(b"\x80\x7f\xff\xff\xff\xff\xff\xff\xff"),
        Some(i64::MAX as isize)
    );
    assert_eq!(parse_number(b"\x80\x80\x00\x00\x00\x00\x00\x00\x00"), None);
    assert_eq!(
        parse_number(b"\xff\x80\x00\x00\x00\x00\x00\x00\x00"),
        Some(i64::MIN as isize)
    );
    assert_eq!(parse_number(b"\xff\x7f\xff\xff\xff\xff\xff\xff\xff"), None);
    assert_eq!(
        parse_number(b"\xf5\xec\xd1\xc7\x7e\x5f\x26\x48\x81\x9f\x8f\x9b"),
        None
    );
}
