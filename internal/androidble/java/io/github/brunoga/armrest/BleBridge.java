package io.github.brunoga.armrest;

import android.bluetooth.BluetoothGatt;
import android.bluetooth.BluetoothGattCallback;
import android.bluetooth.BluetoothGattCharacteristic;
import android.bluetooth.BluetoothGattDescriptor;

/**
 * BleBridge hands Android's GATT callbacks to Go (internal/androidble):
 * Go cannot subclass a Java class, so this one forwards each event, with
 * the handle of the Go connection it belongs to, to native methods the
 * Go side registers. It is compiled to bridge.dex, which the app embeds
 * and loads at run time.
 */
public class BleBridge extends BluetoothGattCallback {
    private final long handle;

    public BleBridge(long handle) {
        this.handle = handle;
    }

    static native void onState(long handle, int status, int state);
    static native void onServices(long handle, int status);
    static native void onMtu(long handle, int mtu, int status);
    static native void onWrite(long handle, int status);
    static native void onDescriptor(long handle, int status);
    static native void onNotify(long handle, byte[] value);

    @Override
    public void onConnectionStateChange(BluetoothGatt gatt, int status, int newState) {
        onState(handle, status, newState);
    }

    @Override
    public void onServicesDiscovered(BluetoothGatt gatt, int status) {
        onServices(handle, status);
    }

    @Override
    public void onMtuChanged(BluetoothGatt gatt, int mtu, int status) {
        onMtu(handle, mtu, status);
    }

    @Override
    public void onCharacteristicWrite(BluetoothGatt gatt, BluetoothGattCharacteristic c, int status) {
        onWrite(handle, status);
    }

    @Override
    public void onDescriptorWrite(BluetoothGatt gatt, BluetoothGattDescriptor d, int status) {
        onDescriptor(handle, status);
    }

    // Android 13 and later.
    @Override
    public void onCharacteristicChanged(BluetoothGatt gatt, BluetoothGattCharacteristic c, byte[] value) {
        onNotify(handle, value);
    }

    // Before Android 13: the value is the characteristic's.
    @Override
    @SuppressWarnings("deprecation")
    public void onCharacteristicChanged(BluetoothGatt gatt, BluetoothGattCharacteristic c) {
        if (android.os.Build.VERSION.SDK_INT < 33) {
            onNotify(handle, c.getValue());
        }
    }
}
