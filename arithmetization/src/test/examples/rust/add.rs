fn add(op1: u8, op2: u8) -> u16 {
    (op1 as u16) + (op2 as u16)
}

fn main() {
    let _ = add(2, 3);
}
