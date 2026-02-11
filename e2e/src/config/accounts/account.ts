export default class Account {
  public readonly privateKey: `0x${string}`;
  public readonly address: `0x${string}`;

  constructor(privateKey: `0x${string}`, address: `0x${string}`) {
    this.privateKey = privateKey;
    this.address = address;
  }
}
