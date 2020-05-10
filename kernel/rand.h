#pragma once
#ifndef RAND_H
#define RAND_H

extern uint64 rand_Retries;

void rand_Init(void);
uint64 rand_Read(uint8* buf, uint64 len);

#endif // RAND_H
