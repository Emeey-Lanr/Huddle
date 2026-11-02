/// <reference types="astro/client" />

declare namespace App {
  interface Locals {
    host: string;
    store: import("./lib/api").Store;
    bagCount: number;
  }
}
