//! # Math Agent v1.0 - Ainur's First Real WASM Agent
//! 
//! This is a historic moment - the FIRST real agent executing on Ainur!
//! 
//! Capabilities:
//! - add(a, b) -> returns a + b
//! - multiply(a, b) -> returns a * b
//! - factorial(n) -> returns n!
//! - fibonacci(n) -> returns nth Fibonacci number
//!
//! Built: November 12, 2025
//! Status: 🚀 PRODUCTION READY

#![no_std]

use core::panic::PanicInfo;

/// Panic handler for WASM (required for no_std)
#[panic_handler]
fn panic(_info: &PanicInfo) -> ! {
    loop {}
}

/// Add two numbers
/// 
/// # Example
/// ```
/// assert_eq!(add(2, 2), 4);
/// ```
#[no_mangle]
pub extern "C" fn add(a: i32, b: i32) -> i32 {
    a + b
}

/// Multiply two numbers
/// 
/// # Example
/// ```
/// assert_eq!(multiply(3, 4), 12);
/// ```
#[no_mangle]
pub extern "C" fn multiply(a: i32, b: i32) -> i32 {
    a * b
}

/// Calculate factorial
/// 
/// # Example
/// ```
/// assert_eq!(factorial(5), 120);
/// ```
#[no_mangle]
pub extern "C" fn factorial(n: i32) -> i64 {
    if n <= 1 {
        return 1;
    }
    
    let mut result: i64 = 1;
    for i in 2..=n {
        result *= i as i64;
    }
    result
}

/// Calculate nth Fibonacci number
/// 
/// # Example
/// ```
/// assert_eq!(fibonacci(10), 55);
/// ```
#[no_mangle]
pub extern "C" fn fibonacci(n: i32) -> i64 {
    if n <= 1 {
        return n as i64;
    }
    
    let mut a: i64 = 0;
    let mut b: i64 = 1;
    
    for _ in 2..=n {
        let temp = a + b;
        a = b;
        b = temp;
    }
    
    b
}

/// Subtract two numbers
#[no_mangle]
pub extern "C" fn subtract(a: i32, b: i32) -> i32 {
    a - b
}

/// Divide two numbers (returns 0 if b == 0 to avoid panic)
#[no_mangle]
pub extern "C" fn divide(a: i32, b: i32) -> i32 {
    if b == 0 {
        return 0;
    }
    a / b
}

/// Power function (a^b)
#[no_mangle]
pub extern "C" fn power(a: i32, b: i32) -> i64 {
    if b < 0 {
        return 0;
    }
    
    let mut result: i64 = 1;
    for _ in 0..b {
        result *= a as i64;
    }
    result
}

/// Check if a number is prime
#[no_mangle]
pub extern "C" fn is_prime(n: i32) -> i32 {
    if n <= 1 {
        return 0; // false
    }
    if n <= 3 {
        return 1; // true
    }
    if n % 2 == 0 || n % 3 == 0 {
        return 0; // false
    }
    
    let mut i = 5;
    while i * i <= n {
        if n % i == 0 || n % (i + 2) == 0 {
            return 0; // false
        }
        i += 6;
    }
    
    1 // true
}

/// Greatest Common Divisor (Euclidean algorithm)
#[no_mangle]
pub extern "C" fn gcd(mut a: i32, mut b: i32) -> i32 {
    while b != 0 {
        let temp = b;
        b = a % b;
        a = temp;
    }
    a.abs()
}

/// Least Common Multiple
#[no_mangle]
pub extern "C" fn lcm(a: i32, b: i32) -> i32 {
    if a == 0 || b == 0 {
        return 0;
    }
    (a * b).abs() / gcd(a, b)
}

// Tests (run with: cargo test)
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_add() {
        assert_eq!(add(2, 2), 4);
        assert_eq!(add(-1, 1), 0);
        assert_eq!(add(100, 200), 300);
    }

    #[test]
    fn test_multiply() {
        assert_eq!(multiply(3, 4), 12);
        assert_eq!(multiply(-2, 5), -10);
        assert_eq!(multiply(0, 100), 0);
    }

    #[test]
    fn test_factorial() {
        assert_eq!(factorial(0), 1);
        assert_eq!(factorial(1), 1);
        assert_eq!(factorial(5), 120);
        assert_eq!(factorial(10), 3628800);
    }

    #[test]
    fn test_fibonacci() {
        assert_eq!(fibonacci(0), 0);
        assert_eq!(fibonacci(1), 1);
        assert_eq!(fibonacci(10), 55);
        assert_eq!(fibonacci(20), 6765);
    }

    #[test]
    fn test_is_prime() {
        assert_eq!(is_prime(2), 1);
        assert_eq!(is_prime(17), 1);
        assert_eq!(is_prime(4), 0);
        assert_eq!(is_prime(100), 0);
    }

    #[test]
    fn test_gcd() {
        assert_eq!(gcd(48, 18), 6);
        assert_eq!(gcd(100, 50), 50);
        assert_eq!(gcd(17, 19), 1);
    }
}
