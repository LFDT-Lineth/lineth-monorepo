import { ResolvedRegister } from "abitype";

export type Hex = `0x${string}`;

export type Address = ResolvedRegister["addressType"];
