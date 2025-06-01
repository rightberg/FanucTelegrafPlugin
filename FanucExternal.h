#ifndef FANUC_EXTERNAL_H
#define FANUC_EXTERNAL_H

#ifdef __cplusplus
extern "C" 
{
#endif
    typedef struct
    {
        unsigned short data;
        short error;
    } UShortDataEx;

    __declspec(dllexport) UShortDataEx GetHandleExternal(const char* address, int port, int timeout);
    __declspec(dllexport) short FreeHandleExternal(unsigned short handle);
    __declspec(dllexport) char* GetFanucJsonDataExternal(const char* json_device, unsigned short handle, short handle_error);

#ifdef __cplusplus
}
#endif

#endif
